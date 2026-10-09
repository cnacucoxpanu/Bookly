package repository

import (
	"context"
	"errors"
	"fmt"

	"bookly/services/auth/internal/model"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrEmailExists = errors.New("email already exists")

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *model.User) error {
	query := `
	INSERT INTO USERS (email, password_hash, name, role)
	VALUES ($1, $2, $3, $4)
	RETURNING id, created_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		user.Email,
		user.PasswordHash,
		user.Name,
		user.Role,
	).Scan(&user.ID, &user.CreatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrEmailExists
		}

		return fmt.Errorf("create user: %w", err)
	}
	return nil
}
