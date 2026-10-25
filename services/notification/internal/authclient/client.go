package authclient

import (
	"context"
	"fmt"
	"time"

	authv1 "bookly/contracts/gen/auth/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const callTimeout = 3 * time.Second

// User is the part of a user profile the notification service needs.
type User struct {
	ID    int64
	Email string
	Name  string
}

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

func (c *Client) GetUser(ctx context.Context, id int64) (*User, error) {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()

	resp, err := c.client.GetUser(ctx, &authv1.GetUserRequest{UserId: id})
	if err != nil {
		return nil, fmt.Errorf("get user %d: %w", id, err)
	}

	return &User{ID: resp.GetUserId(), Email: resp.GetEmail(), Name: resp.GetName()}, nil
}
