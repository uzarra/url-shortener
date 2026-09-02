package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func NewRouter(h *Handler) http.Handler {
	r := chi.NewRouter()
	r.Post("/", h.Shorten)
	r.Get("/{id}", h.Expand)
	return r
}
