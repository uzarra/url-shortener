package middleware

import (
	"compress/gzip"
	"mime"
	"net/http"
	"strings"
)

const (
	ContentEncoding = "Content-Encoding"
	ContentLength   = "Content-Length"
)

var compressibleTypes = map[string]bool{
	"application/json": true,
	"text/html":        true,
}

func Gzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hasGzip(r.Header.Get(ContentEncoding)) {
			zr, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "incorrect gzip body", http.StatusBadRequest)
				return
			}
			defer zr.Close()
			r.Body = zr
			r.Header.Del(ContentEncoding)
			r.Header.Del(ContentLength)
			r.ContentLength = -1
		}

		if !hasGzip(r.Header.Get("Accept-Encoding")) {
			next.ServeHTTP(w, r)
			return
		}
		cw := &compressWriter{ResponseWriter: w}
		defer cw.Close()
		next.ServeHTTP(cw, r)
	})
}

func hasGzip(header string) bool {
	for _, enc := range strings.Split(header, ",") {
		if strings.TrimSpace(strings.SplitN(enc, ";", 2)[0]) == "gzip" {
			return true
		}
	}
	return false
}

type compressWriter struct {
	http.ResponseWriter
	zw          *gzip.Writer
	wroteHeader bool
}

func (c *compressWriter) WriteHeader(statusCode int) {
	if c.wroteHeader {
		return
	}
	c.wroteHeader = true
	mediaType, _, _ := mime.ParseMediaType(c.Header().Get("Content-Type"))
	if compressibleTypes[mediaType] && c.Header().Get(ContentEncoding) == "" {
		c.Header().Set(ContentEncoding, "gzip")
		c.Header().Del(ContentLength)
		c.zw = gzip.NewWriter(c.ResponseWriter)
	}
	c.ResponseWriter.WriteHeader(statusCode)
}

func (c *compressWriter) Write(p []byte) (int, error) {
	if !c.wroteHeader {
		c.WriteHeader(http.StatusOK)
	}
	if c.zw != nil {
		return c.zw.Write(p)
	}
	return c.ResponseWriter.Write(p)
}

func (c *compressWriter) Close() error {
	if c.zw == nil {
		return nil
	}
	return c.zw.Close()
}
