package handler

import (
	"bookly/services/auth/internal/service"
	"encoding/json"
	"errors"
	"log"
	"net/http"
)

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	err := h.authService.Register(
		r.Context(),
		req.Email,
		req.Password,
		req.Name,
	)
	if err != nil {
		if errors.Is(err, service.ErrEmailExists) {
			http.Error(w, "Email already exists", http.StatusConflict)
			return
		}

		log.Printf("Registration failed: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "Registration request recieved",
		"email":   req.Email,
		"name":    req.Name,
	})
}
