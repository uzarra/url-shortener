package config

import (
	"flag"
	"fmt"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	ServerAddr      string `env:"SERVER_ADDRESS"`
	BaseURL         string `env:"BASE_URL"`
	FileStoragePath string `env:"FILE_STORAGE_PATH"`
	DatabaseDSN     string `env:"DATABASE_DSN"`
}

func Load() (*Config, error) {
	var cfg Config
	flag.StringVar(&cfg.ServerAddr, "a", ":8080", "address and port to run server")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080/", "baseURL of shortened url")
	flag.StringVar(&cfg.FileStoragePath, "f", "", "path to file storage")
	flag.StringVar(&cfg.DatabaseDSN, "d", "", "database connection string")
	flag.Parse()
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("parse env: %w", err)
	}
	return &cfg, nil
}
