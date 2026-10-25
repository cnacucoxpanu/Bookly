package config

import (
	"errors"
	"os"
)

type Config struct {
	HTTPAddr     string
	DatabaseURL  string
	AuthGRPCAddr string
	RabbitMQURL  string
}

func Load() (*Config, error) {
	cfg := &Config{
		HTTPAddr:     getEnv("HTTP_ADDR", ":8082"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		AuthGRPCAddr: getEnv("AUTH_GRPC_ADDR", "localhost:9091"),
		RabbitMQURL:  getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
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
