package handler

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/uzarra/url-shortener/internal/api"
)

const maxBodyBytes = 1 << 20

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
	originalURL, err := h.svc.Expand(id)
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
	id, err := h.svc.Shorten(url)
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
