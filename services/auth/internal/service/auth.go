package service

import (
	"context"
	"errors"

	"bookly/services/auth/internal/model"
	"bookly/services/auth/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrEmailExists  = errors.New("email already exists")
)

type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
}

type AuthService struct {
	users UserRepository
}

func NewAuthService(users UserRepository) *AuthService {
	return &AuthService{users: users}
}

func (s *AuthService) Register(
	ctx context.Context,
	email string,
	password string,
	name string,
) error {
	if email == "" || password == "" || name == "" {
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
