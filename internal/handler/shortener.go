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
	"github.com/rs/zerolog"
	"github.com/uzarra/url-shortener/internal/api"
	"github.com/uzarra/url-shortener/internal/auth"
	"github.com/uzarra/url-shortener/internal/model"
	"github.com/uzarra/url-shortener/internal/service"
)

const maxBodyBytes = 1 << 20
const headerContentType = "Content-Type"

type Shortener interface {
	Shorten(ctx context.Context, url string, userID string) (string, error)
	Expand(ctx context.Context, id string) (string, error)
	ShortenBatch(ctx context.Context, batchItems []model.BatchItem, userID string) ([]model.BatchResult, bool, error)
	GetUserURLs(ctx context.Context, userID string) ([]model.UserURL, error)
}

type Pinger interface {
	PingContext(ctx context.Context) error
}

type Deleter interface {
	Delete(ctx context.Context, userID string, ids []string) error
}

type Handler struct {
	svc  Shortener
	del  Deleter
	pngr Pinger
	log  zerolog.Logger
}

func New(svc Shortener, del Deleter, pngr Pinger, log zerolog.Logger) *Handler {
	return &Handler{
		svc:  svc,
		del:  del,
		pngr: pngr,
		log:  log,
	}
}

func (h *Handler) serverError(w http.ResponseWriter, r *http.Request, status int, msg string, err error) {
	h.log.Error().
		Err(err).
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Int("status", status).
		Msg(msg)
	http.Error(w, msg, status)
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
	shortURL, err := h.svc.Shorten(r.Context(), original, userIDFromRequest(r))
	status := http.StatusCreated
	switch {
	case errors.Is(err, service.ErrConflict):
		status = http.StatusConflict
	case err != nil:
		h.serverError(w, r, http.StatusInternalServerError, "failed to shorten url", err)
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
	switch {
	case errors.Is(err, service.ErrDeleted):
		http.Error(w, "url deleted", http.StatusGone)
		return
	case err != nil:
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
	shortURL, err := h.svc.Shorten(r.Context(), url, userIDFromRequest(r))
	status := http.StatusCreated
	switch {
	case errors.Is(err, service.ErrConflict):
		status = http.StatusConflict
	case err != nil:
		h.serverError(w, r, http.StatusInternalServerError, "failed to shorten url", err)
		return
	}
	response := api.ShortenResponse{
		Result: shortURL,
	}
	jsonData, err := json.Marshal(response)
	if err != nil {
		h.serverError(w, r, http.StatusInternalServerError, "failed to marshal response", err)
		return
	}
	w.Header().Set(headerContentType, "application/json")
	w.WriteHeader(status)
	w.Write(jsonData)
}

func (h *Handler) Ping(w http.ResponseWriter, r *http.Request) {
	if h.pngr == nil {
		h.serverError(w, r, http.StatusInternalServerError, "database is not configured", nil)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	if err := h.pngr.PingContext(ctx); err != nil {
		h.serverError(w, r, http.StatusInternalServerError, "database ping failed", err)
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
	batchResults, conflict, err := h.svc.ShortenBatch(r.Context(), batchItems, userIDFromRequest(r))
	if err != nil {
		h.serverError(w, r, http.StatusInternalServerError, "failed to shorten batch", err)
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
		h.serverError(w, r, http.StatusInternalServerError, "failed to marshal response", err)
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

func userIDFromRequest(r *http.Request) string {
	userID, _ := auth.UserIDFromContext(r.Context())
	return userID
}

func (h *Handler) requireUserID(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID, err := auth.UserIDFromContext(r.Context())
	switch {
	case errors.Is(err, auth.ErrUserIDInvalidType):
		h.serverError(w, r, http.StatusInternalServerError, "internal error", err)
		return "", false
	case err != nil:
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return "", false
	}
	return userID, true
}

func (h *Handler) GetUserURLs(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	userURLs, err := h.svc.GetUserURLs(r.Context(), userID)
	if err != nil {
		h.serverError(w, r, http.StatusInternalServerError, "failed to get user urls", err)
		return
	}
	if len(userURLs) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	response := make([]api.UserURLResponseItem, len(userURLs))
	for i, u := range userURLs {
		response[i] = api.UserURLResponseItem{
			ShortURL:    u.ShortURL,
			OriginalURL: u.OriginalURL,
		}
	}
	jsonData, err := json.Marshal(response)
	if err != nil {
		h.serverError(w, r, http.StatusInternalServerError, "failed to marshal response", err)
		return
	}
	w.Header().Set(headerContentType, "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonData)
}

func (h *Handler) DeleteUserURLs(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	contentType := r.Header.Get(headerContentType)
	if mediaType, _, err := mime.ParseMediaType(contentType); err != nil || mediaType != "application/json" {
		http.Error(w, "incorrect content-type", http.StatusBadRequest)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var ids []string
	if err := json.NewDecoder(r.Body).Decode(&ids); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			http.Error(w, "request entity too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "incorrect request body", http.StatusBadRequest)
		return
	}
	if len(ids) == 0 {
		http.Error(w, "empty batch", http.StatusBadRequest)
		return
	}
	err := h.del.Delete(r.Context(), userID, ids)
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		h.log.Debug().Err(err).Str("path", r.URL.Path).Msg("delete request abandoned by client")
		return
	case err != nil:
		h.serverError(w, r, http.StatusServiceUnavailable, "service unavailable", err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
