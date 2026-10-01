package main

import (
	stdlog "log"
	"net/http"

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
	repo, err := repository.NewFileStorage(cfg.FileStoragePath)
	if err != nil {
		stdlog.Fatalf("init file storage: %v", err)
	}
	svc := service.NewShortener(repo, cfg.BaseURL)
	h := handler.New(svc)
	router := handler.NewRouter(h, log)
	log.Info().Str("addr", cfg.ServerAddr).Msg("starting server")
	if err := http.ListenAndServe(cfg.ServerAddr, router); err != nil {
		log.Fatal().Err(err).Msg("server stopped")
	}
}
