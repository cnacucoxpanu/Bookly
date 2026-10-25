package config

import (
	"errors"
	"os"
)

type Config struct {
	DatabaseURL  string
	AuthGRPCAddr string
	RabbitMQURL  string
	Queue        string
}

func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		AuthGRPCAddr: getEnv("AUTH_GRPC_ADDR", "localhost:9091"),
		RabbitMQURL:  getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		Queue:        getEnv("NOTIFICATION_QUEUE", "notifications"),
	}

	if cfg.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is not set")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
