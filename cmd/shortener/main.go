package main

import (
	"database/sql"
	stdlog "log"
	"net/http"

	"github.com/rs/zerolog"
	"github.com/uzarra/url-shortener/internal/config"
	"github.com/uzarra/url-shortener/internal/handler"
	"github.com/uzarra/url-shortener/internal/logger"
	"github.com/uzarra/url-shortener/internal/repository"
	"github.com/uzarra/url-shortener/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		stdlog.Fatalf("load config: %v", err)
	}
	log, err := logger.New("info")
	if err != nil {
		stdlog.Fatalf("init logger: %v", err)
	}
	repo, db, err := newRepository(cfg, log)
	if err != nil {
		log.Fatal().Err(err).Msg("init repository")
	}
	var pinger handler.Pinger
	if db != nil {
		defer db.Close()
		pinger = db
	}
	svc := service.NewShortener(repo, cfg.BaseURL)
	h := handler.New(svc, pinger)
	router := handler.NewRouter(h, log)
	log.Info().Str("addr", cfg.ServerAddr).Msg("starting server")
	if err := http.ListenAndServe(cfg.ServerAddr, router); err != nil {
		log.Fatal().Err(err).Msg("server stopped")
	}
}

func newRepository(cfg *config.Config, log zerolog.Logger) (service.Repository, *sql.DB, error) {
	if cfg.DatabaseDSN != "" {
		db, err := repository.OpenPostgres(cfg.DatabaseDSN)
		if err != nil {
			return nil, nil, err
		}
		repo, err := repository.NewPostgresStorage(db)
		if err != nil {
			db.Close()
			return nil, nil, err
		}
		log.Info().Msg("using postgres storage")
		return repo, db, nil
	}
	if cfg.FileStoragePath != "" {
		repo, err := repository.NewFileStorage(cfg.FileStoragePath)
		log.Info().Msg("using file storage")
		return repo, nil, err
	}
	log.Info().Msg("using in memory storage")
	return repository.NewStorage(), nil, nil
}
