package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/uzarra/url-shortener/internal/api"
)

const maxBodyBytes = 1 << 20

type Shortener interface {
	Shorten(ctx context.Context, url string) (string, error)
	Expand(ctx context.Context, id string) (string, error)
}

type Pinger interface {
	PingContext(ctx context.Context) error
}

type Handler struct {
	svc  Shortener
	pngr Pinger
}

func New(svc Shortener, pngr Pinger) *Handler {
	return &Handler{
		svc:  svc,
		pngr: pngr,
	}
}

func (h *Handler) Shorten(w http.ResponseWriter, r *http.Request) {
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
	id, err := h.svc.Shorten(r.Context(), original)
	if err != nil {
		http.Error(w, "failed to generate id", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(id))
}

func (h *Handler) Expand(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "incorrect id", http.StatusBadRequest)
		return
	}
	originalURL, err := h.svc.Expand(r.Context(), id)
	if err != nil {
		http.Error(w, "no such id", http.StatusNotFound)
		return
	}
	w.Header().Set("Location", originalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
}

func (h *Handler) ShortenInBody(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	if mediaType, _, err := mime.ParseMediaType(contentType); err != nil || mediaType != "application/json" {
		http.Error(w, "incorrect content-type", http.StatusBadRequest)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var request api.ShortenRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			http.Error(w, "request entity too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "incorrect request body", http.StatusBadRequest)
		return
	}
	url := strings.TrimSpace(request.URL)
	if url == "" {
		http.Error(w, "empty url", http.StatusBadRequest)
		return
	}
	id, err := h.svc.Shorten(r.Context(), url)
	if err != nil {
		http.Error(w, "failed to generate id", http.StatusInternalServerError)
		return
	}
	response := api.ShortenResponse{
		Result: id,
	}
	jsonData, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "failed to marshal", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(jsonData)
}

func (h *Handler) Ping(w http.ResponseWriter, r *http.Request) {
	if h.pngr == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	if err := h.pngr.PingContext(ctx); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
