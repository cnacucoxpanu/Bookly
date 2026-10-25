package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"bookly/services/booking/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrSlotTaken        = errors.New("slot is already taken")
	ErrBookingNotFound  = errors.New("booking not found")
	ErrBookingNotActive = errors.New("booking is not active")
)

const (
	// Postgres error codes we care about.
	codeSerializationFailure = "40001"
	codeDeadlockDetected     = "40P01"
	codeForeignKeyViolation  = "23503"
	codeExclusionViolation   = "23P01"

	maxTxAttempts = 5
)

type BookingRepository struct {
	db *pgxpool.Pool
}

func NewBookingRepository(db *pgxpool.Pool) *BookingRepository {
	return &BookingRepository{db: db}
}

// Create books the space for the given period, or returns ErrSlotTaken if the
// period overlaps with another active booking of the same space.
//
// The check and the insert run in one SERIALIZABLE transaction. When two
// requests race for the same slot, Postgres aborts one of them with a
// serialization failure; we retry it, and on the second attempt it sees the
// winner's booking and gets ErrSlotTaken. The exclusion constraint on the
// table guarantees the same thing even if this code is ever bypassed.
func (r *BookingRepository) Create(ctx context.Context, booking *model.Booking) error {
	var err error

	for attempt := 1; attempt <= maxTxAttempts; attempt++ {
		err = r.createOnce(ctx, booking)
		if !isRetryable(err) {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt) * 10 * time.Millisecond):
		}
	}

	return fmt.Errorf("create booking: too many conflicting transactions: %w", err)
}

func (r *BookingRepository) createOnce(ctx context.Context, booking *model.Booking) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var taken bool
	err = tx.QueryRow(ctx, `
	SELECT EXISTS (
		SELECT 1
		FROM bookings
		WHERE space_id = $1
		  AND status = 'active'
		  AND start_time < $3
		  AND end_time > $2
	)
	`, booking.SpaceID, booking.StartTime, booking.EndTime).Scan(&taken)
	if err != nil {
		return fmt.Errorf("check overlapping bookings: %w", err)
	}
	if taken {
		return ErrSlotTaken
	}

	err = tx.QueryRow(ctx, `
	INSERT INTO bookings (space_id, user_id, start_time, end_time)
	VALUES ($1, $2, $3, $4)
	RETURNING id, status, created_at
	`, booking.SpaceID, booking.UserID, booking.StartTime, booking.EndTime).
		Scan(&booking.ID, &booking.Status, &booking.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case codeExclusionViolation:
				return ErrSlotTaken
			case codeForeignKeyViolation:
				return ErrSpaceNotFound
			}
		}
		return fmt.Errorf("insert booking: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit booking: %w", err)
	}
	return nil
}

func (r *BookingRepository) GetByID(ctx context.Context, id int64) (*model.Booking, error) {
	query := `
	SELECT b.id, b.space_id, s.name, b.user_id, b.start_time, b.end_time, b.status, b.created_at
	FROM bookings b
	JOIN spaces s ON s.id = b.space_id
	WHERE b.id = $1
	`

	booking := &model.Booking{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&booking.ID,
		&booking.SpaceID,
		&booking.SpaceName,
		&booking.UserID,
		&booking.StartTime,
		&booking.EndTime,
		&booking.Status,
		&booking.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrBookingNotFound
		}
		return nil, fmt.Errorf("get booking: %w", err)
	}
	return booking, nil
}

func (r *BookingRepository) ListByUser(ctx context.Context, userID int64) ([]model.Booking, error) {
	rows, err := r.db.Query(ctx, `
	SELECT b.id, b.space_id, s.name, b.user_id, b.start_time, b.end_time, b.status, b.created_at
	FROM bookings b
	JOIN spaces s ON s.id = b.space_id
	WHERE b.user_id = $1
	ORDER BY b.start_time DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list bookings: %w", err)
	}
	defer rows.Close()

	bookings := []model.Booking{}
	for rows.Next() {
		var b model.Booking
		err := rows.Scan(
			&b.ID,
			&b.SpaceID,
			&b.SpaceName,
			&b.UserID,
			&b.StartTime,
			&b.EndTime,
			&b.Status,
			&b.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan booking: %w", err)
		}
		bookings = append(bookings, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list bookings: %w", err)
	}
	return bookings, nil
}

// Cancel marks an active booking as cancelled. The status check is part of the
// UPDATE, so two parallel cancellations cannot both succeed.
func (r *BookingRepository) Cancel(ctx context.Context, id int64) error {
	tag, err := r.db.Exec(ctx, `
	UPDATE bookings
	SET status = 'cancelled'
	WHERE id = $1 AND status = 'active'
	`, id)
	if err != nil {
		return fmt.Errorf("cancel booking: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrBookingNotActive
	}
	return nil
}

func isRetryable(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == codeSerializationFailure || pgErr.Code == codeDeadlockDetected
}
