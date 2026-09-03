package handler

import (
	"io"
	"net/http"
	"strings"
)

type Shortener interface {
	Shorten(url string) (string, error)
	Expand(id string) (string, error)
}

type Handler struct {
	svc Shortener
}

func New(svc Shortener) *Handler {
	return &Handler{
		svc: svc,
	}
}

func (h *Handler) Shorten(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	if contentType != "text/plain" {
		http.Error(w, "incorrect content-type", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		http.Error(w, "incorrect request body", http.StatusBadRequest)
		return
	}
	original := strings.TrimSpace(string(body))
	if original == "" {
		http.Error(w, "empty request body", http.StatusBadRequest)
		return
	}
	id, err := h.svc.Shorten(original)
	if err != nil {
		http.Error(w, "couldn`t generate id", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(id))
}

func (h *Handler) Expand(w http.ResponseWriter, r *http.Request) {
	id := strings.Trim(r.URL.Path, "/")
	if id == "" {
		http.Error(w, "incorrect id", http.StatusBadRequest)
		return
	}
	originalURL, err := h.svc.Expand(id)
	if err != nil {
		http.Error(w, "no such id", http.StatusBadRequest)
		return
	}
	w.Header().Set("Location", originalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
}
