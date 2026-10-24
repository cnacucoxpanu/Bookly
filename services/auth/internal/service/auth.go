package service

import (
	"context"
	"errors"
	"strings"

	"bookly/services/auth/internal/model"
	"bookly/services/auth/internal/repository"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidInput       = errors.New("invalid input")
	ErrEmailExists        = errors.New("email already exists")
	ErrInvalidCredentials = errors.New("invalid email or password")
)

type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	GetByEmail(ctx context.Context, email string) (*model.User, error)
}

type AuthService struct {
	users  UserRepository
	tokens *TokenService
}

func NewAuthService(users UserRepository, tokens *TokenService) *AuthService {
	return &AuthService{
		users:  users,
		tokens: tokens,
	}
}

func (s *AuthService) Register(
	ctx context.Context,
	email string,
	password string,
	name string,
) error {
	email = strings.TrimSpace(strings.ToLower(email))
	name = strings.TrimSpace(name)

	if email == "" || password == "" || name == "" || len(password) < 8 {
		return ErrInvalidInput
	}

	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		return err
	}

	user := &model.User{
		Email:        email,
		PasswordHash: string(passwordHash),
		Name:         name,
		Role:         "user",
	}

	err = s.users.Create(ctx, user)
	if errors.Is(err, repository.ErrEmailExists) {
		return ErrEmailExists
	}

	return err
}

func (s *AuthService) Login(
	ctx context.Context,
	email string,
	password string,
) (*model.User, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))

	if email == "" || password == "" {
		return nil, "", ErrInvalidCredentials
	}

	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", ErrInvalidCredentials
		}

		return nil, "", err
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash),
		[]byte(password),
	); err != nil {
		return nil, "", ErrInvalidCredentials
	}

	token, err := s.tokens.Generate(user.ID, user.Role)
	if err != nil {
		return nil, "", err
	}

	return user, token, nil
}
