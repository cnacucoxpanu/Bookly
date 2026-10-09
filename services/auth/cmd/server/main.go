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
	"bookly/services/auth/internal/repository"
	"bookly/services/auth/internal/service"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load("../../.env"); err != nil {
		log.Println("No .env file found; using environment variables")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := database.Connect(ctx, databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect database: %v", err)
	}
	defer db.Close()

	log.Println("Connected to PostgreSQL")

	healthHandler := handler.NewHealthHandler()
	userRepository := repository.NewUserRepository(db)
	authService := service.NewAuthService(userRepository)
	authHandler := handler.NewAuthHandler(authService)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler.Check)
	mux.HandleFunc("POST /register", authHandler.Register)

	server := &http.Server{
		Addr:    ":8081",
		Handler: mux,
	}

	log.Println("Auth Service started on :8081")

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("HTTP server error: %v", err)
	}
}
