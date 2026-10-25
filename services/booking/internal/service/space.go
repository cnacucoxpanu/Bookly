package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"bookly/services/booking/internal/model"
	"bookly/services/booking/internal/repository"
)

const RoleAdmin = "admin"

type SpaceRepository interface {
	Create(ctx context.Context, space *model.Space) error
	List(ctx context.Context) ([]model.Space, error)
	GetByID(ctx context.Context, id int64) (*model.Space, error)
}

type SpaceService struct {
	spaces SpaceRepository
}

func NewSpaceService(spaces SpaceRepository) *SpaceService {
	return &SpaceService{spaces: spaces}
}

func (s *SpaceService) Create(
	ctx context.Context,
	role string,
	name string,
	description string,
	capacity int,
) (*model.Space, error) {
	if role != RoleAdmin {
		return nil, ErrForbidden
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrInvalidInput)
	}
	if capacity <= 0 {
		return nil, fmt.Errorf("%w: capacity must be positive", ErrInvalidInput)
	}

	space := &model.Space{
		Name:        name,
		Description: strings.TrimSpace(description),
		Capacity:    capacity,
	}

	if err := s.spaces.Create(ctx, space); err != nil {
		if errors.Is(err, repository.ErrSpaceExists) {
			return nil, ErrSpaceExists
		}
		return nil, err
	}
	return space, nil
}

func (s *SpaceService) List(ctx context.Context) ([]model.Space, error) {
	return s.spaces.List(ctx)
}

func (s *SpaceService) Get(ctx context.Context, id int64) (*model.Space, error) {
	space, err := s.spaces.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrSpaceNotFound) {
			return nil, ErrSpaceNotFound
		}
		return nil, err
	}
	return space, nil
}
