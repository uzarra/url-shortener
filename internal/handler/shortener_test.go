package handler

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/uzarra/url-shortener/internal/model"
	"github.com/uzarra/url-shortener/internal/service"
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

func TestHandler_Shorten_Conflict(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/",
		bytes.NewReader([]byte(`http://test.ru`)))
	request.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	handler := New(&conflictTestShortener{}, &testPinger{})
	handler.Shorten(rec, request)
	mediaType, _, _ := mime.ParseMediaType(rec.Header().Get("Content-Type"))
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "http://localhost:8080/existing", rec.Body.String())
	assert.Equal(t, "text/plain", mediaType)
}

func TestHandler_Shorten_ConflictLookupFails(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/",
		bytes.NewReader([]byte(`http://test.ru`)))
	request.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	handler := New(&conflictTestShortener{lookupErr: errors.New("bad")}, &testPinger{})
	handler.Shorten(rec, request)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
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

func TestHandler_ShortenInBody_Conflict(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/shorten",
		bytes.NewReader([]byte(`{"url":"http://test.ru"}`)))
	request.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler := New(&conflictTestShortener{}, &testPinger{})
	handler.ShortenInBody(rec, request)
	mediaType, _, _ := mime.ParseMediaType(rec.Header().Get("Content-Type"))
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.JSONEq(t, `{"result":"http://localhost:8080/existing"}`, rec.Body.String())
	assert.Equal(t, "application/json", mediaType)
}

func TestHandler_ShortenInBody_ConflictLookupFails(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/shorten",
		bytes.NewReader([]byte(`{"url":"http://test.ru"}`)))
	request.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler := New(&conflictTestShortener{lookupErr: errors.New("bad")}, &testPinger{})
	handler.ShortenInBody(rec, request)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandler_ShortenBatch_BadRequest(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		want        int
	}{
		{name: "bad content-type", contentType: "text/plain",
			body: `[{"correlation_id":"1","original_url":"http://test.ru"}]`, want: http.StatusBadRequest},
		{name: "bad json", contentType: "application/json", body: `not json`, want: http.StatusBadRequest},
		{name: "not an array", contentType: "application/json",
			body: `{"correlation_id":"1","original_url":"http://test.ru"}`, want: http.StatusBadRequest},
		{name: "empty batch", contentType: "application/json", body: `[]`, want: http.StatusBadRequest},
		{name: "empty url", contentType: "application/json",
			body: `[{"correlation_id":"1","original_url":"http://test.ru"},{"correlation_id":"2","original_url":"  "}]`,
			want: http.StatusBadRequest},
		{name: "too large body", contentType: "application/json",
			body: `[{"correlation_id":"1","original_url":"http://test.ru/` + strings.Repeat("a", maxBodyBytes) + `"}]`,
			want: http.StatusRequestEntityTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/shorten/batch",
				bytes.NewReader([]byte(tt.body)))
			request.Header.Set("Content-Type", tt.contentType)
			rec := httptest.NewRecorder()
			handler := New(newTestShortener(), &testPinger{})
			handler.ShortenBatch(rec, request)
			assert.Equal(t, tt.want, rec.Code)
		})
	}
}

func TestHandler_ShortenBatch_ShortenFails(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/shorten/batch",
		bytes.NewReader([]byte(`[{"correlation_id":"1","original_url":"http://test.ru"}]`)))
	request.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler := New(newBadTestShortener(), &testPinger{})
	handler.ShortenBatch(rec, request)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "[")
}

func TestHandler_ShortenBatch(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/shorten/batch",
		bytes.NewReader([]byte(`[
			{"correlation_id":"a1","original_url":"http://test.ru"},
			{"correlation_id":"b2","original_url":"http://ya.ru"}
		]`)))
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	rec := httptest.NewRecorder()
	handler := New(newTestShortener(), &testPinger{})
	handler.ShortenBatch(rec, request)
	mediaType, _, _ := mime.ParseMediaType(rec.Header().Get("Content-Type"))
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "application/json", mediaType)
	assert.JSONEq(t, `[
		{"correlation_id":"a1","short_url":"http://localhost:8080/a1"},
		{"correlation_id":"b2","short_url":"http://localhost:8080/b2"}
	]`, rec.Body.String())
}

func TestHandler_ShortenBatch_Gzip(t *testing.T) {
	var body bytes.Buffer
	zw := gzip.NewWriter(&body)
	_, _ = zw.Write([]byte(`[{"correlation_id":"a1","original_url":"http://test.ru"}]`))
	_ = zw.Close()

	request := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", &body)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Content-Encoding", "gzip")
	request.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	router := NewRouter(New(newTestShortener(), &testPinger{}), zerolog.Nop())
	router.ServeHTTP(rec, request)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
	zr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatalf("gzip.NewReader() error = %v", err)
	}
	decoded, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("read gzip body: %v", err)
	}
	assert.JSONEq(t, `[{"correlation_id":"a1","short_url":"http://localhost:8080/a1"}]`, string(decoded))
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

func (t *testShortener) Shorten(_ context.Context, _ string) (string, error) {
	return "http://localhost:8080/qwe123qw", nil
}

func (t *testShortener) Expand(_ context.Context, id string) (string, error) {
	return "https://yandex.ru", nil
}

func (t *testShortener) ShortenBatch(_ context.Context, batchItems []model.BatchItem) ([]model.BatchResult, error) {
	results := make([]model.BatchResult, len(batchItems))
	for i, item := range batchItems {
		results[i] = model.BatchResult{
			CorrelationID: item.CorrelationID,
			ShortURL:      "http://localhost:8080/" + item.CorrelationID,
		}
	}
	return results, nil
}

func (t *testShortener) GetShortURLByOriginalURL(_ context.Context, _ string) (string, error) {
	return "http://localhost:8080/qwe123qw", nil
}

type badTestShortener struct {
}

func newBadTestShortener() Shortener {
	return &badTestShortener{}
}

func (t *badTestShortener) Shorten(_ context.Context, _ string) (string, error) {
	return "", errors.New("bad")
}

func (t *badTestShortener) Expand(_ context.Context, _ string) (string, error) {
	return "", errors.New("bad")
}

func (t *badTestShortener) ShortenBatch(_ context.Context, _ []model.BatchItem) ([]model.BatchResult, error) {
	return nil, errors.New("bad")
}

func (t *badTestShortener) GetShortURLByOriginalURL(_ context.Context, _ string) (string, error) {
	return "", errors.New("bad")
}

type conflictTestShortener struct {
	testShortener
	lookupErr error
}

func (t *conflictTestShortener) Shorten(_ context.Context, _ string) (string, error) {
	return "", service.ErrConflict
}

func (t *conflictTestShortener) GetShortURLByOriginalURL(_ context.Context, _ string) (string, error) {
	if t.lookupErr != nil {
		return "", t.lookupErr
	}
	return "http://localhost:8080/existing", nil
}

type testPinger struct {
	err error
}

func (p *testPinger) PingContext(_ context.Context) error {
	return p.err
}
