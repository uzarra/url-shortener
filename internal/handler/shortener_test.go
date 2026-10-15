package handler

import (
	"bytes"
	"context"
	"errors"
	"mime"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

func TestHandler_Expand_BadId(t *testing.T) {
	badIDRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "")
	badIDRequest = badIDRequest.WithContext(context.WithValue(badIDRequest.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	handler := New(newTestShortener(), &testPinger{})
	handler.Expand(rec, badIDRequest)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_Expand_ExpandFails(t *testing.T) {
	badIDRequest := httptest.NewRequest(http.MethodGet, "/qwe123qw", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "qwe123qw")
	badIDRequest = badIDRequest.WithContext(context.WithValue(badIDRequest.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	handler := New(newBadTestShortener(), &testPinger{})
	handler.Expand(rec, badIDRequest)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandler_Expand(t *testing.T) {
	goodRequest := httptest.NewRequest(http.MethodGet, "/qwe123qw", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "qwe123qw")
	goodRequest = goodRequest.WithContext(context.WithValue(goodRequest.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	handler := New(newTestShortener(), &testPinger{})
	handler.Expand(rec, goodRequest)
	assert.Equal(t, http.StatusTemporaryRedirect, rec.Code)
	assert.Equal(t, "https://yandex.ru", rec.Header().Get("Location"))
}

func TestHandler_Shorten_BadBody(t *testing.T) {
	noBodyRequest := httptest.NewRequest(http.MethodPost, "/", nil)
	noBodyRequest.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	handler := New(newTestShortener(), &testPinger{})
	handler.Shorten(rec, noBodyRequest)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_Shorten_EmptyBody(t *testing.T) {
	emptyBodyRequest := httptest.NewRequest(http.MethodPost, "/",
		bytes.NewReader([]byte(``)))
	emptyBodyRequest.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	handler := New(newTestShortener(), &testPinger{})
	handler.Shorten(rec, emptyBodyRequest)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_Shorten_ShortenFails(t *testing.T) {
	correctRequest := httptest.NewRequest(http.MethodPost, "/",
		bytes.NewReader([]byte(`http://test.ru`)))
	correctRequest.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	handler := New(newBadTestShortener(), &testPinger{})
	handler.Shorten(rec, correctRequest)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandler_Shorten(t *testing.T) {
	correctRequest := httptest.NewRequest(http.MethodPost, "/",
		bytes.NewReader([]byte(`http://test.ru`)))
	correctRequest.Header.Set("Content-Type", "text/plain; charset=utf-8")
	rec := httptest.NewRecorder()
	handler := New(newTestShortener(), &testPinger{})
	handler.Shorten(rec, correctRequest)
	mediaType, _, _ := mime.ParseMediaType(rec.Header().Get("Content-Type"))
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "http://localhost:8080/qwe123qw", rec.Body.String())
	assert.Equal(t, "text/plain", mediaType)
}

func TestHandler_ShortenInBody_BadContentType(t *testing.T) {
	badContentTypeRequest := httptest.NewRequest(http.MethodPost, "/api/shorten",
		bytes.NewReader([]byte(`{"url":"http://test.ru"}`)))
	badContentTypeRequest.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	handler := New(newTestShortener(), &testPinger{})
	handler.ShortenInBody(rec, badContentTypeRequest)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_ShortenInBody_BadJSON(t *testing.T) {
	badJSONRequest := httptest.NewRequest(http.MethodPost, "/api/shorten",
		bytes.NewReader([]byte(`not json`)))
	badJSONRequest.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler := New(newTestShortener(), &testPinger{})
	handler.ShortenInBody(rec, badJSONRequest)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_ShortenInBody_EmptyURL(t *testing.T) {
	emptyURLRequest := httptest.NewRequest(http.MethodPost, "/api/shorten",
		bytes.NewReader([]byte(`{"url":"  "}`)))
	emptyURLRequest.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler := New(newTestShortener(), &testPinger{})
	handler.ShortenInBody(rec, emptyURLRequest)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_ShortenInBody_TooLargeBody(t *testing.T) {
	largeBody := `{"url":"http://test.ru/` + strings.Repeat("a", maxBodyBytes) + `"}`
	tooLargeRequest := httptest.NewRequest(http.MethodPost, "/api/shorten",
		bytes.NewReader([]byte(largeBody)))
	tooLargeRequest.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler := New(newTestShortener(), &testPinger{})
	handler.ShortenInBody(rec, tooLargeRequest)
	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
}

func TestHandler_ShortenInBody_ShortenFails(t *testing.T) {
	correctRequest := httptest.NewRequest(http.MethodPost, "/api/shorten",
		bytes.NewReader([]byte(`{"url":"http://test.ru"}`)))
	correctRequest.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler := New(newBadTestShortener(), &testPinger{})
	handler.ShortenInBody(rec, correctRequest)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandler_ShortenInBody(t *testing.T) {
	correctRequest := httptest.NewRequest(http.MethodPost, "/api/shorten",
		bytes.NewReader([]byte(`{"url":"http://test.ru"}`)))
	correctRequest.Header.Set("Content-Type", "application/json; charset=utf-8")
	rec := httptest.NewRecorder()
	handler := New(newTestShortener(), &testPinger{})
	handler.ShortenInBody(rec, correctRequest)
	mediaType, _, _ := mime.ParseMediaType(rec.Header().Get("Content-Type"))
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.JSONEq(t, `{"result":"http://localhost:8080/qwe123qw"}`, rec.Body.String())
	assert.Equal(t, "application/json", mediaType)
}

func TestHandler_Ping(t *testing.T) {
	tests := []struct {
		name   string
		pinger Pinger
		want   int
	}{
		{
			name:   "ok",
			pinger: &testPinger{},
			want:   http.StatusOK,
		},
		{
			name:   "fail",
			pinger: &testPinger{err: errors.New("connection refused")},
			want:   http.StatusInternalServerError,
		},
		{
			name:   "nil pinger",
			pinger: nil,
			want:   http.StatusInternalServerError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/ping", nil)
			rec := httptest.NewRecorder()
			handler := New(newTestShortener(), tt.pinger)
			handler.Ping(rec, request)
			assert.Equal(t, tt.want, rec.Code)
		})
	}
}

func TestNew(t *testing.T) {
	type args struct {
		svc  Shortener
		pngr Pinger
	}
	tests := []struct {
		name string
		args args
		want *Handler
	}{
		{
			name: "ok",
			args: args{
				svc:  newTestShortener(),
				pngr: &testPinger{},
			},
			want: &Handler{
				svc:  newTestShortener(),
				pngr: &testPinger{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := New(tt.args.svc, tt.args.pngr); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("New() = %v, want %v", got, tt.want)
			}
		})
	}
}

type testShortener struct {
}

func newTestShortener() Shortener {
	return &testShortener{}
}

func (t *testShortener) Shorten(_ context.Context, url string) (string, error) {
	return "http://localhost:8080/qwe123qw", nil
}

func (t *testShortener) Expand(_ context.Context, id string) (string, error) {
	return "https://yandex.ru", nil
}

type badTestShortener struct {
}

func newBadTestShortener() Shortener {
	return &badTestShortener{}
}

func (t *badTestShortener) Shorten(_ context.Context, url string) (string, error) {
	return "", errors.New("bad")
}

func (t *badTestShortener) Expand(_ context.Context, id string) (string, error) {
	return "", errors.New("bad")
}

type testPinger struct {
	err error
}

func (p *testPinger) PingContext(ctx context.Context) error {
	return p.err
}
