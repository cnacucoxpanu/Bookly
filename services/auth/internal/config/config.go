package config

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr    string
	GRPCAddr    string
	DatabaseURL string
	JWTSecret   string
	TokenTTL    time.Duration
}

func Load() (*Config, error) {
	if err := loadEnvFile(); err != nil {
		return nil, err
	}

	cfg := &Config{
		HTTPAddr:    getEnv("HTTP_ADDR", ":8081"),
		GRPCAddr:    getEnv("GRPC_ADDR", ":9091"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
	}

	if cfg.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is not set")
	}
	if len(cfg.JWTSecret) < 32 {
		return nil, errors.New("JWT_SECRET must contain at least 32 characters")
	}

	ttl, err := time.ParseDuration(getEnv("TOKEN_TTL", "15m"))
	if err != nil {
		return nil, fmt.Errorf("parse TOKEN_TTL: %w", err)
	}
	cfg.TokenTTL = ttl

	return cfg, nil
}

// loadEnvFile reads a .env file if there is one. Variables that are already
// set in the environment win over the file, and a missing file is fine
// (inside a container everything comes from the environment).
func loadEnvFile() error {
	for _, path := range []string{".env", "../../.env"} {
		err := godotenv.Load(path)
		if err == nil {
			return nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("load %s: %w", path, err)
		}
	}
	return nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
