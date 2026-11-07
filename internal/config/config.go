package config

import (
	"flag"
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

const (
	defaultDeleteBatchSize     = 100
	defaultDeleteFlushInterval = time.Second
	defaultDeleteMaxWorkers    = 10
)

type Config struct {
	ServerAddr          string        `env:"SERVER_ADDRESS"`
	BaseURL             string        `env:"BASE_URL"`
	FileStoragePath     string        `env:"FILE_STORAGE_PATH"`
	DatabaseDSN         string        `env:"DATABASE_DSN"`
	SecretKey           string        `env:"SECRET_KEY"`
	DeleteBatchSize     int           `env:"DELETE_BATCH_SIZE"`
	DeleteFlushInterval time.Duration `env:"DELETE_FLUSH_INTERVAL"`
	DeleteMaxWorkers    int           `env:"DELETE_MAX_WORKERS"`
}

func Load() (*Config, error) {
	var cfg Config
	flag.StringVar(&cfg.ServerAddr, "a", ":8080", "address and port to run server")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080/", "baseURL of shortened url")
	flag.StringVar(&cfg.FileStoragePath, "f", "", "path to file storage")
	flag.StringVar(&cfg.DatabaseDSN, "d", "", "database connection string")
	flag.StringVar(&cfg.SecretKey, "k", "dev-secret", "secret key for auth tokens")
	flag.IntVar(&cfg.DeleteBatchSize, "delete-batch-size", defaultDeleteBatchSize, "max urls in one delete batch")
	flag.DurationVar(&cfg.DeleteFlushInterval, "delete-flush-interval", defaultDeleteFlushInterval, "how often to flush a partial delete batch")
	flag.IntVar(&cfg.DeleteMaxWorkers, "delete-max-workers", defaultDeleteMaxWorkers, "max concurrent delete writer goroutines")
	flag.Parse()
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("parse env: %w", err)
	}
	if cfg.SecretKey == "" {
		return nil, fmt.Errorf("secret key must not be empty")
	}
	if cfg.DeleteBatchSize <= 0 {
		return nil, fmt.Errorf("delete batch size must be positive, got %d", cfg.DeleteBatchSize)
	}
	if cfg.DeleteFlushInterval <= 0 {
		return nil, fmt.Errorf("delete flush interval must be positive, got %s", cfg.DeleteFlushInterval)
	}
	if cfg.DeleteMaxWorkers <= 0 {
		return nil, fmt.Errorf("delete max workers must be positive, got %d", cfg.DeleteMaxWorkers)
	}
	return &cfg, nil
}
