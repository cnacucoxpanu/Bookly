package authclient

import (
	"context"
	"errors"
	"fmt"
	"time"

	authv1 "bookly/contracts/gen/auth/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

var ErrInvalidToken = errors.New("invalid token")

const callTimeout = 3 * time.Second

// Client talks to the Auth service over gRPC.
type Client struct {
	conn   *grpc.ClientConn
	client authv1.AuthServiceClient
}

func New(addr string) (*Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("create auth grpc client: %w", err)
	}

	return &Client{conn: conn, client: authv1.NewAuthServiceClient(conn)}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

// ValidateToken returns the user id and role for a valid access token and
// ErrInvalidToken when Auth rejects it.
func (c *Client) ValidateToken(ctx context.Context, token string) (int64, string, error) {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()

	resp, err := c.client.ValidateToken(ctx, &authv1.ValidateTokenRequest{Token: token})
	if err != nil {
		if status.Code(err) == codes.Unauthenticated {
			return 0, "", ErrInvalidToken
		}
		return 0, "", fmt.Errorf("validate token: %w", err)
	}

	return resp.GetUserId(), resp.GetRole(), nil
}
