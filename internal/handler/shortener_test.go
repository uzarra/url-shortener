package handler

import (
	"bytes"
	"errors"
	"mime"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandler_Expand_BadId(t *testing.T) {
	badIDRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler := New(newTestShortener())
	handler.Expand(rec, badIDRequest)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_Expand_ExpandFails(t *testing.T) {
	badIDRequest := httptest.NewRequest(http.MethodGet, "/qwe123qw", nil)
	rec := httptest.NewRecorder()
	handler := New(newBadTestShortener())
	handler.Expand(rec, badIDRequest)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_Expand(t *testing.T) {
	goodRequest := httptest.NewRequest(http.MethodGet, "/qwe123qw", nil)
	rec := httptest.NewRecorder()
	handler := New(newTestShortener())
	handler.Expand(rec, goodRequest)
	assert.Equal(t, http.StatusTemporaryRedirect, rec.Code)
	assert.Equal(t, "https://yandex.ru", rec.Header().Get("Location"))
}

func TestHandler_Shorten_BadContentType(t *testing.T) {
	badContentTypeRequest := httptest.NewRequest(http.MethodPost, "/", nil)
	badContentTypeRequest.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler := New(newTestShortener())
	handler.Shorten(rec, badContentTypeRequest)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_Shorten_BadBody(t *testing.T) {
	noBodyRequest := httptest.NewRequest(http.MethodPost, "/", nil)
	noBodyRequest.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	handler := New(newTestShortener())
	handler.Shorten(rec, noBodyRequest)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_Shorten_EmptyBody(t *testing.T) {
	emptyBodyRequest := httptest.NewRequest(http.MethodPost, "/",
		bytes.NewReader([]byte(``)))
	emptyBodyRequest.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	handler := New(newTestShortener())
	handler.Shorten(rec, emptyBodyRequest)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_Shorten_ShortenFails(t *testing.T) {
	correctRequest := httptest.NewRequest(http.MethodPost, "/",
		bytes.NewReader([]byte(`http://test.ru`)))
	correctRequest.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	handler := New(newBadTestShortener())
	handler.Shorten(rec, correctRequest)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_Shorten(t *testing.T) {
	correctRequest := httptest.NewRequest(http.MethodPost, "/",
		bytes.NewReader([]byte(`http://test.ru`)))
	correctRequest.Header.Set("Content-Type", "text/plain; charset=utf-8")
	rec := httptest.NewRecorder()
	handler := New(newTestShortener())
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
	handler := New(newTestShortener())
	handler.ShortenInBody(rec, badContentTypeRequest)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_ShortenInBody_BadJSON(t *testing.T) {
	badJSONRequest := httptest.NewRequest(http.MethodPost, "/api/shorten",
		bytes.NewReader([]byte(`not json`)))
	badJSONRequest.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler := New(newTestShortener())
	handler.ShortenInBody(rec, badJSONRequest)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_ShortenInBody_EmptyURL(t *testing.T) {
	emptyURLRequest := httptest.NewRequest(http.MethodPost, "/api/shorten",
		bytes.NewReader([]byte(`{"url":"  "}`)))
	emptyURLRequest.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler := New(newTestShortener())
	handler.ShortenInBody(rec, emptyURLRequest)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_ShortenInBody_ShortenFails(t *testing.T) {
	correctRequest := httptest.NewRequest(http.MethodPost, "/api/shorten",
		bytes.NewReader([]byte(`{"url":"http://test.ru"}`)))
	correctRequest.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler := New(newBadTestShortener())
	handler.ShortenInBody(rec, correctRequest)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_ShortenInBody(t *testing.T) {
	correctRequest := httptest.NewRequest(http.MethodPost, "/api/shorten",
		bytes.NewReader([]byte(`{"url":"http://test.ru"}`)))
	correctRequest.Header.Set("Content-Type", "application/json; charset=utf-8")
	rec := httptest.NewRecorder()
	handler := New(newTestShortener())
	handler.ShortenInBody(rec, correctRequest)
	mediaType, _, _ := mime.ParseMediaType(rec.Header().Get("Content-Type"))
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.JSONEq(t, `{"result":"http://localhost:8080/qwe123qw"}`, rec.Body.String())
	assert.Equal(t, "application/json", mediaType)
}

func TestNew(t *testing.T) {
	type args struct {
		svc Shortener
	}
	tests := []struct {
		name string
		args args
		want *Handler
	}{
		{
			name: "ok",
			args: args{
				svc: newTestShortener(),
			},
			want: &Handler{
				svc: newTestShortener(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := New(tt.args.svc); !reflect.DeepEqual(got, tt.want) {
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

func (t *testShortener) Shorten(url string) (string, error) {
	return "http://localhost:8080/qwe123qw", nil
}

func (t *testShortener) Expand(id string) (string, error) {
	return "https://yandex.ru", nil
}

type badTestShortener struct {
}

func newBadTestShortener() Shortener {
	return &badTestShortener{}
}

func (t *badTestShortener) Shorten(url string) (string, error) {
	return "", errors.New("bad")
}

func (t *badTestShortener) Expand(id string) (string, error) {
	return "", errors.New("bad")
}
