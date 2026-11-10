package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandler_ServerErrorsAreLogged(t *testing.T) {
	tests := []struct {
		name       string
		handler    func(h *Handler) http.HandlerFunc
		svc        Shortener
		del        Deleter
		pngr       Pinger
		method     string
		path       string
		ctype      string
		body       string
		withUser   bool
		wantStatus int
		wantErr    string
	}{
		{
			name:       "shorten fails",
			handler:    func(h *Handler) http.HandlerFunc { return h.Shorten },
			svc:        newBadTestShortener(),
			method:     http.MethodPost,
			path:       "/",
			ctype:      "text/plain",
			body:       "http://yandex.ru",
			wantStatus: http.StatusInternalServerError,
			wantErr:    "bad",
		},
		{
			name:       "shorten json fails",
			handler:    func(h *Handler) http.HandlerFunc { return h.ShortenInBody },
			svc:        newBadTestShortener(),
			method:     http.MethodPost,
			path:       "/api/shorten",
			ctype:      "application/json",
			body:       `{"url":"http://yandex.ru"}`,
			wantStatus: http.StatusInternalServerError,
			wantErr:    "bad",
		},
		{
			name:       "shorten batch fails",
			handler:    func(h *Handler) http.HandlerFunc { return h.ShortenBatch },
			svc:        newBadTestShortener(),
			method:     http.MethodPost,
			path:       "/api/shorten/batch",
			ctype:      "application/json",
			body:       `[{"correlation_id":"1","original_url":"http://yandex.ru"}]`,
			wantStatus: http.StatusInternalServerError,
			wantErr:    "bad",
		},
		{
			name:       "get user urls fails",
			handler:    func(h *Handler) http.HandlerFunc { return h.GetUserURLs },
			svc:        newBadTestShortener(),
			method:     http.MethodGet,
			path:       "/api/user/urls",
			withUser:   true,
			wantStatus: http.StatusInternalServerError,
			wantErr:    "bad",
		},
		{
			name:       "ping fails",
			handler:    func(h *Handler) http.HandlerFunc { return h.Ping },
			svc:        newTestShortener(),
			pngr:       &testPinger{err: errors.New("db down")},
			method:     http.MethodGet,
			path:       "/ping",
			wantStatus: http.StatusInternalServerError,
			wantErr:    "db down",
		},
		{
			name:       "ping without database",
			handler:    func(h *Handler) http.HandlerFunc { return h.Ping },
			svc:        newTestShortener(),
			method:     http.MethodGet,
			path:       "/ping",
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "deleter unavailable",
			handler:    func(h *Handler) http.HandlerFunc { return h.DeleteUserURLs },
			svc:        newTestShortener(),
			del:        &testDeleter{err: errors.New("deleter is closed")},
			method:     http.MethodDelete,
			path:       "/api/user/urls",
			ctype:      "application/json",
			body:       `["abc"]`,
			withUser:   true,
			wantStatus: http.StatusServiceUnavailable,
			wantErr:    "deleter is closed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logBuf bytes.Buffer
			var pngr Pinger
			if tt.pngr != nil {
				pngr = tt.pngr
			}
			h := New(tt.svc, tt.del, pngr, zerolog.New(&logBuf))

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			if tt.ctype != "" {
				req.Header.Set(headerContentType, tt.ctype)
			}
			if tt.withUser {
				req = withUser(req, handlerTestUser)
			}
			rec := httptest.NewRecorder()

			tt.handler(h)(rec, req)

			require.Equal(t, tt.wantStatus, rec.Code)
			var entry map[string]any
			require.NoError(t, json.Unmarshal(logBuf.Bytes(), &entry), "expected one JSON log line, got %q", logBuf.String())
			assert.Equal(t, "error", entry["level"])
			assert.Equal(t, tt.method, entry["method"])
			assert.Equal(t, tt.path, entry["path"])
			assert.EqualValues(t, tt.wantStatus, entry["status"])
			assert.NotEmpty(t, entry["message"])
			if tt.wantErr != "" {
				assert.Equal(t, tt.wantErr, entry["error"])
			}
		})
	}
}

func TestHandler_ClientErrorsAreNotLogged(t *testing.T) {
	var logBuf bytes.Buffer
	h := New(newTestShortener(), &testDeleter{}, &testPinger{}, zerolog.New(&logBuf))

	req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader("not json"))
	req.Header.Set(headerContentType, "application/json")
	rec := httptest.NewRecorder()
	h.ShortenInBody(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Empty(t, logBuf.String())
}
