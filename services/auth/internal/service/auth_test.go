package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"bookly/services/auth/internal/model"
	"bookly/services/auth/internal/repository"
)

type fakeUsers struct {
	users  map[string]*model.User
	nextID int64
}

func newFakeUsers() *fakeUsers {
	return &fakeUsers{users: map[string]*model.User{}, nextID: 1}
}

func (f *fakeUsers) Create(_ context.Context, user *model.User) error {
	if _, ok := f.users[user.Email]; ok {
		return repository.ErrEmailExists
	}
	user.ID = f.nextID
	f.nextID++
	f.users[user.Email] = user
	return nil
}

func (f *fakeUsers) GetByEmail(_ context.Context, email string) (*model.User, error) {
	user, ok := f.users[email]
	if !ok {
		return nil, repository.ErrUserNotFound
	}
	return user, nil
}

func (f *fakeUsers) GetByID(_ context.Context, id int64) (*model.User, error) {
	for _, user := range f.users {
		if user.ID == id {
			return user, nil
		}
	}
	return nil, repository.ErrUserNotFound
}

func newTestService() (*AuthService, *fakeUsers) {
	users := newFakeUsers()
	tokens := NewTokenService("test-secret-test-secret-test-secret", time.Minute)
	return NewAuthService(users, tokens), users
}

func TestRegisterValidatesInput(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		password string
		userName string
	}{
		{"empty email", "", "password123", "Anna"},
		{"blank email", "   ", "password123", "Anna"},
		{"empty password", "anna@example.com", "", "Anna"},
		{"short password", "anna@example.com", "short", "Anna"},
		{"empty name", "anna@example.com", "password123", "  "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, users := newTestService()

			err := svc.Register(context.Background(), tt.email, tt.password, tt.userName)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("got %v, want ErrInvalidInput", err)
			}
			if len(users.users) != 0 {
				t.Fatal("user must not be created")
			}
		})
	}
}

func TestRegisterStoresHashedPasswordAndNormalizesEmail(t *testing.T) {
	svc, users := newTestService()

	if err := svc.Register(context.Background(), "  Anna@Example.COM ", "password123", " Anna "); err != nil {
		t.Fatal(err)
	}

	user, ok := users.users["anna@example.com"]
	if !ok {
		t.Fatal("user was not stored under the normalized email")
	}
	if user.PasswordHash == "password123" || user.PasswordHash == "" {
		t.Fatal("password must be stored as a hash")
	}
	if user.Name != "Anna" || user.Role != "user" {
		t.Fatalf("unexpected user: %+v", user)
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	if err := svc.Register(ctx, "anna@example.com", "password123", "Anna"); err != nil {
		t.Fatal(err)
	}

	err := svc.Register(ctx, "ANNA@example.com", "password123", "Anna 2")
	if !errors.Is(err, ErrEmailExists) {
		t.Fatalf("got %v, want ErrEmailExists", err)
	}
}

func TestLogin(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	if err := svc.Register(ctx, "anna@example.com", "password123", "Anna"); err != nil {
		t.Fatal(err)
	}

	t.Run("success", func(t *testing.T) {
		user, token, err := svc.Login(ctx, "Anna@example.com", "password123")
		if err != nil {
			t.Fatal(err)
		}

		userID, role, err := svc.ValidateToken(token)
		if err != nil {
			t.Fatalf("token from login must be valid: %v", err)
		}
		if userID != user.ID || role != "user" {
			t.Fatalf("token has user %d/%s, want %d/user", userID, role, user.ID)
		}
	})

	// A wrong password and an unknown email must look the same to the caller,
	// otherwise the endpoint reveals which emails are registered.
	t.Run("wrong password", func(t *testing.T) {
		_, _, err := svc.Login(ctx, "anna@example.com", "wrong-password")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("got %v, want ErrInvalidCredentials", err)
		}
	})

	t.Run("unknown email", func(t *testing.T) {
		_, _, err := svc.Login(ctx, "nobody@example.com", "password123")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("got %v, want ErrInvalidCredentials", err)
		}
	})

	t.Run("empty fields", func(t *testing.T) {
		_, _, err := svc.Login(ctx, "", "")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("got %v, want ErrInvalidCredentials", err)
		}
	})
}

func TestValidateTokenRejectsGarbage(t *testing.T) {
	svc, _ := newTestService()

	if _, _, err := svc.ValidateToken("not-a-token"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("got %v, want ErrInvalidToken", err)
	}
}

func TestGetUser(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	if err := svc.Register(ctx, "anna@example.com", "password123", "Anna"); err != nil {
		t.Fatal(err)
	}

	user, err := svc.GetUser(ctx, 1)
	if err != nil || user.Email != "anna@example.com" {
		t.Fatalf("got %+v, %v", user, err)
	}

	if _, err := svc.GetUser(ctx, 42); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("got %v, want ErrUserNotFound", err)
	}
}
