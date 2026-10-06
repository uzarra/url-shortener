package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func gzipBytes(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := zw.Write(data)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func echoHandler(contentType string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(http.StatusCreated)
		w.Write(body)
	})
}

func TestGzip_DecompressRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(gzipBytes(t, []byte(`{"url":"http://test.ru"}`))))
	req.Header.Set("Content-Encoding", "gzip")
	rec := httptest.NewRecorder()
	Gzip(echoHandler("application/json")).ServeHTTP(rec, req)
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, `{"url":"http://test.ru"}`, rec.Body.String())
	assert.Empty(t, rec.Header().Get("Content-Encoding"))
}

func TestGzip_BadGzipRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("not gzip")))
	req.Header.Set("Content-Encoding", "gzip")
	rec := httptest.NewRecorder()
	Gzip(echoHandler("application/json")).ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGzip_CompressResponse(t *testing.T) {
	for _, contentType := range []string{"application/json", "text/html; charset=utf-8"} {
		t.Run(contentType, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("payload")))
			req.Header.Set("Accept-Encoding", "gzip, deflate")
			rec := httptest.NewRecorder()
			Gzip(echoHandler(contentType)).ServeHTTP(rec, req)
			assert.Equal(t, http.StatusCreated, rec.Code)
			assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
			zr, err := gzip.NewReader(rec.Body)
			require.NoError(t, err)
			body, err := io.ReadAll(zr)
			require.NoError(t, err)
			assert.Equal(t, "payload", string(body))
		})
	}
}

func TestGzip_SkipNonCompressibleType(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("payload")))
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	Gzip(echoHandler("text/plain")).ServeHTTP(rec, req)
	assert.Empty(t, rec.Header().Get("Content-Encoding"))
	assert.Equal(t, "payload", rec.Body.String())
}

func TestGzip_NoAcceptEncoding(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("payload")))
	rec := httptest.NewRecorder()
	Gzip(echoHandler("application/json")).ServeHTTP(rec, req)
	assert.Empty(t, rec.Header().Get("Content-Encoding"))
	assert.Equal(t, "payload", rec.Body.String())
}
