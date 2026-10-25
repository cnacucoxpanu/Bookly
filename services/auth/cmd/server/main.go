package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	authv1 "bookly/contracts/gen/auth/v1"
	"bookly/services/auth/docs"
	"bookly/services/auth/internal/config"
	"bookly/services/auth/internal/database"
	"bookly/services/auth/internal/grpcserver"
	"bookly/services/auth/internal/handler"
	"bookly/services/auth/internal/middleware"
	"bookly/services/auth/internal/repository"
	"bookly/services/auth/internal/service"
	"bookly/services/auth/migrations"

	"google.golang.org/grpc"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	db, err := database.Connect(connectCtx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	log.Println("Connected to PostgreSQL")

	if err := database.Migrate(cfg.DatabaseURL, migrations.FS); err != nil {
		return err
	}

	userRepository := repository.NewUserRepository(db)
	tokenService := service.NewTokenService(cfg.JWTSecret, cfg.TokenTTL)
	authService := service.NewAuthService(userRepository, tokenService)
	authHandler := handler.NewAuthHandler(authService)
	healthHandler := handler.NewHealthHandler()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler.Check)
	docs.Register(mux)
	mux.HandleFunc("POST /register", authHandler.Register)
	mux.HandleFunc("POST /login", authHandler.Login)
	mux.Handle(
		"GET /me",
		middleware.Auth(tokenService)(http.HandlerFunc(authHandler.Me)),
	)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	grpcListener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer()
	authv1.RegisterAuthServiceServer(grpcServer, grpcserver.New(authService))

	serverErr := make(chan error, 2)
	go func() {
		log.Printf("Auth HTTP server started on %s", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()
	go func() {
		log.Printf("Auth gRPC server started on %s", cfg.GRPCAddr)
		if err := grpcServer.Serve(grpcListener); err != nil {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		log.Println("Shutting down")
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()

	grpcServer.GracefulStop()

	return server.Shutdown(shutdownCtx)
}
