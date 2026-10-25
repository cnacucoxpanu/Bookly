package grpcserver

import (
	"context"
	"errors"
	"log"

	authv1 "bookly/contracts/gen/auth/v1"
	"bookly/services/auth/internal/model"
	"bookly/services/auth/internal/service"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AuthService interface {
	ValidateToken(token string) (userID int64, role string, err error)
	GetUser(ctx context.Context, id int64) (*model.User, error)
}

type Server struct {
	authv1.UnimplementedAuthServiceServer
	auth AuthService
}

func New(auth AuthService) *Server {
	return &Server{auth: auth}
}

func (s *Server) ValidateToken(
	_ context.Context,
	req *authv1.ValidateTokenRequest,
) (*authv1.ValidateTokenResponse, error) {
	userID, role, err := s.auth.ValidateToken(req.GetToken())
	if err != nil {
		if errors.Is(err, service.ErrInvalidToken) {
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}

		log.Printf("grpc ValidateToken failed: %v", err)
		return nil, status.Error(codes.Internal, "internal error")
	}

	return &authv1.ValidateTokenResponse{UserId: userID, Role: role}, nil
}

func (s *Server) GetUser(
	ctx context.Context,
	req *authv1.GetUserRequest,
) (*authv1.GetUserResponse, error) {
	user, err := s.auth.GetUser(ctx, req.GetUserId())
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, "user not found")
		}

		log.Printf("grpc GetUser failed: %v", err)
		return nil, status.Error(codes.Internal, "internal error")
	}

	return &authv1.GetUserResponse{
		UserId: user.ID,
		Email:  user.Email,
		Name:   user.Name,
		Role:   user.Role,
	}, nil
}
