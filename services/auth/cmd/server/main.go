package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"bookly/services/auth/internal/database"
	"bookly/services/auth/internal/handler"
	"bookly/services/auth/internal/middleware"
	"bookly/services/auth/internal/repository"
	"bookly/services/auth/internal/service"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Overload("../../.env"); err != nil {
		log.Fatalf("Failed to load .env: %v", err)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if len(jwtSecret) < 32 {
		log.Fatal("JWT_SECRET must contain at least 32 characters")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := database.Connect(ctx, databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect database: %v", err)
	}
	defer db.Close()

	log.Println("Connected to PostgreSQL")

	userRepository := repository.NewUserRepository(db)
	tokenService := service.NewTokenService(
		jwtSecret,
		15*time.Minute,
	)
	authService := service.NewAuthService(
		userRepository,
		tokenService,
	)
	authHandler := handler.NewAuthHandler(authService)
	healthHandler := handler.NewHealthHandler()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler.Check)
	mux.HandleFunc("POST /register", authHandler.Register)
	mux.HandleFunc("POST /login", authHandler.Login)

	mux.Handle(
		"GET /me",
		middleware.Auth(tokenService)(http.HandlerFunc(authHandler.Me)),
	)

	server := &http.Server{
		Addr:              ":8081",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Println("Auth Service started on :8081")

	if err := server.ListenAndServe(); err != nil &&
		!errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("HTTP server error: %v", err)
	}
}
