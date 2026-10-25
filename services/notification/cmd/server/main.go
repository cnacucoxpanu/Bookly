package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bookly/services/notification/internal/authclient"
	"bookly/services/notification/internal/config"
	"bookly/services/notification/internal/consumer"
	"bookly/services/notification/internal/database"
	"bookly/services/notification/internal/repository"
	"bookly/services/notification/internal/service"
	"bookly/services/notification/migrations"
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

	notificationService := service.NewNotificationService(
		repository.NewNotificationRepository(db),
		authClient,
	)

	return consumer.New(cfg.RabbitMQURL, cfg.Queue, notificationService).Run(ctx)
}
