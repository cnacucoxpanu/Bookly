package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bookly/contracts/events"
	"bookly/services/booking/internal/authclient"
	"bookly/services/booking/internal/config"
	"bookly/services/booking/internal/database"
	"bookly/services/booking/internal/handler"
	"bookly/services/booking/internal/middleware"
	"bookly/services/booking/internal/repository"
	"bookly/services/booking/internal/service"
	"bookly/services/booking/migrations"
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

	authClient, err := authclient.New(cfg.AuthGRPCAddr)
	if err != nil {
		return err
	}
	defer authClient.Close()

	spaceService := service.NewSpaceService(repository.NewSpaceRepository(db))
	spaceHandler := handler.NewSpaceHandler(spaceService)
	bookingService := service.NewBookingService(
		repository.NewBookingRepository(db),
		repository.NewSpaceRepository(db),
		nopPublisher{},
	)
	bookingHandler := handler.NewBookingHandler(bookingService)
	requireAuth := middleware.Auth(authClient)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /spaces", spaceHandler.List)
	mux.HandleFunc("GET /spaces/{id}", spaceHandler.Get)
	mux.Handle("POST /spaces", requireAuth(http.HandlerFunc(spaceHandler.Create)))
	mux.Handle("POST /bookings", requireAuth(http.HandlerFunc(bookingHandler.Create)))
	mux.Handle("GET /bookings", requireAuth(http.HandlerFunc(bookingHandler.ListMine)))
	mux.Handle("GET /bookings/{id}", requireAuth(http.HandlerFunc(bookingHandler.Get)))
	mux.Handle("DELETE /bookings/{id}", requireAuth(http.HandlerFunc(bookingHandler.Cancel)))

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("Booking Service started on %s", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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

	return server.Shutdown(shutdownCtx)
}

// nopPublisher drops events until the message broker is wired in.
type nopPublisher struct{}

func (nopPublisher) Publish(context.Context, string, events.BookingEvent) error {
	return nil
}
