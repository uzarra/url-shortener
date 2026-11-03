package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uzarra/url-shortener/internal/api"
	"github.com/uzarra/url-shortener/internal/auth"
	"github.com/uzarra/url-shortener/internal/model"
	"github.com/uzarra/url-shortener/internal/repository"
	"github.com/uzarra/url-shortener/internal/service"
)

const handlerTestUser = "11111111-1111-4111-8111-111111111111"

type recordingShortener struct {
	testShortener
	userURLs  []model.UserURL
	userErr   error
	gotUserID string
}

func (s *recordingShortener) Shorten(ctx context.Context, url string, userID string) (string, error) {
	s.gotUserID = userID
	return s.testShortener.Shorten(ctx, url, userID)
}

func (s *recordingShortener) ShortenBatch(ctx context.Context, items []model.BatchItem, userID string) ([]model.BatchResult, bool, error) {
	s.gotUserID = userID
	return s.testShortener.ShortenBatch(ctx, items, userID)
}

func (s *recordingShortener) GetUserURLs(_ context.Context, userID string) ([]model.UserURL, error) {
	s.gotUserID = userID
	return s.userURLs, s.userErr
}

func withUser(r *http.Request, userID string) *http.Request {
	return r.WithContext(auth.WithUserID(r.Context(), userID))
}

func TestHandler_GetUserURLs(t *testing.T) {
	tests := []struct {
		name       string
		userID     string
		urls       []model.UserURL
		err        error
		wantStatus int
		wantBody   []api.UserURLResponseItem
	}{
		{
			name:       "no user in context",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "no urls",
			userID:     handlerTestUser,
			urls:       []model.UserURL{},
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "service error",
			userID:     handlerTestUser,
			err:        errors.New("db is down"),
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:   "user urls",
			userID: handlerTestUser,
			urls: []model.UserURL{
				{ShortURL: "http://localhost:8080/abc", OriginalURL: "http://yandex.ru"},
				{ShortURL: "http://localhost:8080/def", OriginalURL: "http://ya.ru"},
			},
			wantStatus: http.StatusOK,
			wantBody: []api.UserURLResponseItem{
				{ShortURL: "http://localhost:8080/abc", OriginalURL: "http://yandex.ru"},
				{ShortURL: "http://localhost:8080/def", OriginalURL: "http://ya.ru"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &recordingShortener{userURLs: tt.urls, userErr: tt.err}
			req := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
			if tt.userID != "" {
				req = withUser(req, tt.userID)
			}
			rec := httptest.NewRecorder()

			New(svc, &testPinger{}).GetUserURLs(rec, req)

			require.Equal(t, tt.wantStatus, rec.Code)
			if tt.userID != "" {
				assert.Equal(t, tt.userID, svc.gotUserID)
			}
			switch tt.wantStatus {
			case http.StatusNoContent:
				assert.Empty(t, rec.Body.String())
			case http.StatusOK:
				assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
				var got []api.UserURLResponseItem
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
				assert.Equal(t, tt.wantBody, got)
			}
		})
	}
}

func TestHandler_GetUserURLs_JSONFieldNames(t *testing.T) {
	svc := &recordingShortener{userURLs: []model.UserURL{{ShortURL: "http://s", OriginalURL: "http://o"}}}
	rec := httptest.NewRecorder()

	New(svc, &testPinger{}).GetUserURLs(rec, withUser(httptest.NewRequest(http.MethodGet, "/api/user/urls", nil), handlerTestUser))

	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `[{"short_url":"http://s","original_url":"http://o"}]`, rec.Body.String())
}

func TestHandler_PassUserIDToService(t *testing.T) {
	tests := []struct {
		name        string
		method      func(h *Handler) http.HandlerFunc
		path        string
		body        string
		contentType string
	}{
		{name: "POST /", method: func(h *Handler) http.HandlerFunc { return h.Shorten },
			path: "/", body: "http://yandex.ru"},
		{name: "POST /api/shorten", method: func(h *Handler) http.HandlerFunc { return h.ShortenInBody },
			path: "/api/shorten", body: `{"url":"http://yandex.ru"}`, contentType: "application/json"},
		{name: "POST /api/shorten/batch", method: func(h *Handler) http.HandlerFunc { return h.ShortenBatch },
			path: "/api/shorten/batch", body: `[{"correlation_id":"1","original_url":"http://yandex.ru"}]`, contentType: "application/json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &recordingShortener{}
			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			rec := httptest.NewRecorder()

			tt.method(New(svc, &testPinger{}))(rec, withUser(req, handlerTestUser))

			require.Equal(t, http.StatusCreated, rec.Code)
			assert.Equal(t, handlerTestUser, svc.gotUserID)
		})
	}
}

type e2eClient struct {
	t      *testing.T
	router http.Handler
	cookie *http.Cookie
}

func newE2ERouter(t *testing.T) (http.Handler, *auth.Manager) {
	t.Helper()
	m := auth.NewManager([]byte("handler-test-key"), time.Hour)
	svc := service.NewShortener(repository.NewStorage(), "http://localhost:8080/")
	return NewRouter(New(svc, &testPinger{}), zerolog.Nop(), m), m
}

func (c *e2eClient) do(method, path, body, contentType string) *httptest.ResponseRecorder {
	c.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.cookie != nil {
		req.AddCookie(c.cookie)
	}
	rec := httptest.NewRecorder()
	c.router.ServeHTTP(rec, req)
	for _, ck := range rec.Result().Cookies() {
		c.cookie = ck
	}
	return rec
}

func TestE2E_UserURLs(t *testing.T) {
	router, _ := newE2ERouter(t)
	alice := &e2eClient{t: t, router: router}
	bob := &e2eClient{t: t, router: router}

	rec := alice.do(http.MethodPost, "/", "http://alice-1.ru", "text/plain")
	require.Equal(t, http.StatusCreated, rec.Code)
	require.NotNil(t, alice.cookie, "first request must issue a cookie")
	short1 := rec.Body.String()

	rec = alice.do(http.MethodPost, "/api/shorten/batch",
		`[{"correlation_id":"1","original_url":"http://alice-2.ru"}]`, "application/json")
	require.Equal(t, http.StatusCreated, rec.Code)
	var batch []api.ShortenBatchResponseItem
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &batch))

	rec = bob.do(http.MethodPost, "/api/shorten", `{"url":"http://bob.ru"}`, "application/json")
	require.Equal(t, http.StatusCreated, rec.Code)

	rec = alice.do(http.MethodGet, "/api/user/urls", "", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var got []api.UserURLResponseItem
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, []api.UserURLResponseItem{
		{ShortURL: short1, OriginalURL: "http://alice-1.ru"},
		{ShortURL: batch[0].ShortURL, OriginalURL: "http://alice-2.ru"},
	}, got)
}

func TestE2E_UserURLs_NewUserGetsNoContent(t *testing.T) {
	router, _ := newE2ERouter(t)
	client := &e2eClient{t: t, router: router}

	rec := client.do(http.MethodGet, "/api/user/urls", "", "")

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.NotNil(t, client.cookie, "user without cookie must get one")
}

func TestE2E_UserURLs_CookieWithoutUserID(t *testing.T) {
	router, m := newE2ERouter(t)
	token, err := m.Build("")
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
	req.AddCookie(&http.Cookie{Name: "url-shortener.token", Value: token})
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Empty(t, rec.Result().Cookies(), "signed cookie without id must not be replaced")
}
