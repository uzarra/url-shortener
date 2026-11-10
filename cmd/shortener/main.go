package main

import (
	"context"
	"database/sql"
	"errors"
	stdlog "log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/uzarra/url-shortener/internal/auth"
	"github.com/uzarra/url-shortener/internal/config"
	"github.com/uzarra/url-shortener/internal/handler"
	"github.com/uzarra/url-shortener/internal/logger"
	"github.com/uzarra/url-shortener/internal/repository"
	"github.com/uzarra/url-shortener/internal/service"
)

const (
	shutdownTimeout = 10 * time.Second
)

type storage interface {
	service.Repository
	service.URLDeleter
}

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
	deleter := service.NewDeleter(repo, log, cfg.DeleteBatchSize, cfg.DeleteFlushInterval, cfg.DeleteMaxWorkers)
	h := handler.New(svc, deleter, pinger, log)
	authManager := auth.NewManager([]byte(cfg.SecretKey), auth.TokenTTL)
	router := handler.NewRouter(h, log, authManager)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{
		Addr:    cfg.ServerAddr,
		Handler: router,
	}
	serverErr := make(chan error, 1)
	go func() {
		log.Info().Str("addr", cfg.ServerAddr).Msg("starting server")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case <-ctx.Done():
		log.Info().Msg("shutdown signal received")
	case err := <-serverErr:
		if err != nil {
			log.Error().Err(err).Msg("server failed")
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("server shutdown")
	}
	deleter.Close()
	log.Info().Msg("server stopped")
}

func newRepository(cfg *config.Config, log zerolog.Logger) (storage, *sql.DB, error) {
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
