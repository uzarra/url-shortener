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
	"github.com/uzarra/url-shortener/internal/model"
	"github.com/uzarra/url-shortener/internal/service"
)

const maxBodyBytes = 1 << 20
const headerContentType = "Content-Type"

type Shortener interface {
	Shorten(ctx context.Context, url string) (string, error)
	Expand(ctx context.Context, id string) (string, error)
	ShortenBatch(ctx context.Context, batchItems []model.BatchItem) ([]model.BatchResult, bool, error)
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
	shortURL, err := h.svc.Shorten(r.Context(), original)
	status := http.StatusCreated
	switch {
	case errors.Is(err, service.ErrConflict):
		status = http.StatusConflict
	case err != nil:
		http.Error(w, "failed to generate id", http.StatusInternalServerError)
		return
	}
	w.Header().Set(headerContentType, "text/plain")
	w.WriteHeader(status)
	w.Write([]byte(shortURL))
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
	contentType := r.Header.Get(headerContentType)
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
	shortURL, err := h.svc.Shorten(r.Context(), url)
	status := http.StatusCreated
	switch {
	case errors.Is(err, service.ErrConflict):
		status = http.StatusConflict
	case err != nil:
		http.Error(w, "failed to generate id", http.StatusInternalServerError)
		return
	}
	response := api.ShortenResponse{
		Result: shortURL,
	}
	jsonData, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "failed to marshal", http.StatusInternalServerError)
		return
	}
	w.Header().Set(headerContentType, "application/json")
	w.WriteHeader(status)
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

func (h *Handler) ShortenBatch(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get(headerContentType)
	if mediaType, _, err := mime.ParseMediaType(contentType); err != nil || mediaType != "application/json" {
		http.Error(w, "incorrect content-type", http.StatusBadRequest)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var request []api.ShortenBatchRequestItem
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			http.Error(w, "request entity too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "incorrect request body", http.StatusBadRequest)
		return
	}
	if len(request) == 0 {
		http.Error(w, "empty batch", http.StatusBadRequest)
		return
	}

	batchItems := make([]model.BatchItem, len(request))
	for i, requestItem := range request {
		url := strings.TrimSpace(requestItem.OriginalURL)
		if url == "" {
			http.Error(w, "empty url", http.StatusBadRequest)
			return
		}
		batchItems[i] = model.BatchItem{
			CorrelationID: requestItem.CorrelationID,
			OriginalURL:   url,
		}
	}
	batchResults, conflict, err := h.svc.ShortenBatch(r.Context(), batchItems)
	if err != nil {
		http.Error(w, "failed to generate id", http.StatusInternalServerError)
		return
	}
	results := make([]api.ShortenBatchResponseItem, len(batchResults))
	for i, batchItem := range batchResults {
		results[i] = api.ShortenBatchResponseItem{
			CorrelationID: batchItem.CorrelationID,
			ShortURL:      batchItem.ShortURL,
		}
	}
	jsonData, err := json.Marshal(results)
	if err != nil {
		http.Error(w, "failed to marshal", http.StatusInternalServerError)
		return
	}
	status := http.StatusCreated
	if conflict {
		status = http.StatusConflict
	}
	w.Header().Set(headerContentType, "application/json")
	w.WriteHeader(status)
	w.Write(jsonData)
}
