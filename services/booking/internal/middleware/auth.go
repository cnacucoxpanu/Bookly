package middleware

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"

	"bookly/services/booking/internal/authclient"
)

type contextKey string

const (
	userIDKey contextKey = "user_id"
	roleKey   contextKey = "role"
)

type TokenValidator interface {
	ValidateToken(ctx context.Context, token string) (userID int64, role string, err error)
}

// Auth lets a request through only when it carries a valid bearer token.
// The user id and role are stored in the request context.
func Auth(validator TokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			token = strings.TrimSpace(token)
			if !found || token == "" {
				writeError(w, http.StatusUnauthorized, "missing or malformed Authorization header")
				return
			}

			userID, role, err := validator.ValidateToken(r.Context(), token)
			if err != nil {
				if errors.Is(err, authclient.ErrInvalidToken) {
					writeError(w, http.StatusUnauthorized, "invalid token")
					return
				}

				log.Printf("token validation failed: %v", err)
				writeError(w, http.StatusServiceUnavailable, "auth service is unavailable")
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, userID)
			ctx = context.WithValue(ctx, roleKey, role)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserID(ctx context.Context) int64 {
	id, _ := ctx.Value(userIDKey).(int64)
	return id
}

func Role(ctx context.Context) string {
	role, _ := ctx.Value(roleKey).(string)
	return role
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":"` + message + `"}`))
}
