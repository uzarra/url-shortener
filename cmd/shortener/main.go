package main

import (
	"net/http"

	"github.com/uzarra/url-shortener/internal/config"
	"github.com/uzarra/url-shortener/internal/handler"
	"github.com/uzarra/url-shortener/internal/repository"
	"github.com/uzarra/url-shortener/internal/service"
)

func main() {
	cfg := config.Load()
	repo := repository.NewStorage()
	svc := service.NewShortener(repo, cfg.BaseURL)
	h := handler.New(svc)
	router := handler.NewRouter(h)
	if err := http.ListenAndServe(cfg.ServerAddr, router); err != nil {
		panic(err)
	}
}
