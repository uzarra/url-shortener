package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"github.com/uzarra/url-shortener/internal/auth"
	"github.com/uzarra/url-shortener/internal/middleware"
)

func NewRouter(h *Handler, log zerolog.Logger, m *auth.Manager) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger(log))
	r.Use(middleware.Gzip)
	r.Use(middleware.Auth(m))
	r.Post("/", h.Shorten)
	r.Get("/{id}", h.Expand)
	r.Post("/api/shorten", h.ShortenInBody)
	r.Post("/api/shorten/batch", h.ShortenBatch)
	r.Get("/api/user/urls", h.GetUserURLs)
	r.Get("/ping", h.Ping)
	return r
}
