package repository

import (
	"context"
	"errors"
	"fmt"

	"bookly/services/booking/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrSpaceNotFound = errors.New("space not found")
	ErrSpaceExists   = errors.New("space already exists")
)

type SpaceRepository struct {
	db *pgxpool.Pool
}

func NewSpaceRepository(db *pgxpool.Pool) *SpaceRepository {
	return &SpaceRepository{db: db}
}

func (r *SpaceRepository) Create(ctx context.Context, space *model.Space) error {
	query := `
	INSERT INTO spaces (name, description, capacity)
	VALUES ($1, $2, $3)
	RETURNING id, created_at
	`

	err := r.db.QueryRow(ctx, query, space.Name, space.Description, space.Capacity).
		Scan(&space.ID, &space.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrSpaceExists
		}
		return fmt.Errorf("create space: %w", err)
	}
	return nil
}

func (r *SpaceRepository) List(ctx context.Context) ([]model.Space, error) {
	rows, err := r.db.Query(ctx, `
	SELECT id, name, description, capacity, created_at
	FROM spaces
	ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("list spaces: %w", err)
	}
	defer rows.Close()

	spaces := []model.Space{}
	for rows.Next() {
		var s model.Space
		if err := rows.Scan(&s.ID, &s.Name, &s.Description, &s.Capacity, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan space: %w", err)
		}
		spaces = append(spaces, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list spaces: %w", err)
	}
	return spaces, nil
}

func (r *SpaceRepository) GetByID(ctx context.Context, id int64) (*model.Space, error) {
	query := `
	SELECT id, name, description, capacity, created_at
	FROM spaces
	WHERE id = $1
	`

	space := &model.Space{}
	err := r.db.QueryRow(ctx, query, id).
		Scan(&space.ID, &space.Name, &space.Description, &space.Capacity, &space.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSpaceNotFound
		}
		return nil, fmt.Errorf("get space: %w", err)
	}
	return space, nil
}
