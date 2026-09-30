package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"github.com/uzarra/url-shortener/internal/middleware"
)

func NewRouter(h *Handler, log zerolog.Logger) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger(log))
	r.Post("/", h.Shorten)
	r.Get("/{id}", h.Expand)
	r.Post("/api/shorten", h.ShortenInBody)
	return r
}
