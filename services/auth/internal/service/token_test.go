package service

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret-test-secret-test-secret"

func TestTokenRoundTrip(t *testing.T) {
	tokens := NewTokenService(testSecret, time.Minute)

	token, err := tokens.Generate(42, "admin")
	if err != nil {
		t.Fatal(err)
	}

	claims, err := tokens.Validate(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "42" || claims.Role != "admin" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestTokenRejected(t *testing.T) {
	tokens := NewTokenService(testSecret, time.Minute)

	expired, err := NewTokenService(testSecret, -time.Minute).Generate(1, "user")
	if err != nil {
		t.Fatal(err)
	}

	otherSecret, err := NewTokenService("another-secret-another-secret-123", time.Minute).Generate(1, "user")
	if err != nil {
		t.Fatal(err)
	}

	// A token signed with a different algorithm must not be accepted even if
	// the signature is otherwise valid.
	wrongMethod, err := jwt.NewWithClaims(jwt.SigningMethodHS512, UserClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		},
	}).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]string{
		"empty":        "",
		"garbage":      "abc.def.ghi",
		"expired":      expired,
		"other secret": otherSecret,
		"wrong method": wrongMethod,
	}

	for name, token := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := tokens.Validate(token); !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("got %v, want ErrInvalidToken", err)
			}
		})
	}
}
