//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/uzarra/url-shortener/internal/api"
)

var baseURL = strings.TrimRight(getenv("E2E_BASE_URL", "http://localhost:8080"), "/")

var client = &http.Client{
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
	Timeout: 5 * time.Second,
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func TestMain(m *testing.M) {
	if err := waitReady(30 * time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "service at %s is not ready: %v\n", baseURL, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func waitReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := client.Get(baseURL + "/ping")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			err = fmt.Errorf("ping status %d", resp.StatusCode)
		}
		lastErr = err
		time.Sleep(500 * time.Millisecond)
	}
	return lastErr
}

func uniqueURL(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("https://practicum.yandex.ru/e2e/%d", time.Now().UnixNano())
}

func do(t *testing.T, method, path, contentType string, body []byte) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, baseURL+path, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp, string(data)
}

func shortPath(t *testing.T, shortURL string) string {
	t.Helper()
	u, err := url.Parse(strings.TrimSpace(shortURL))
	if err != nil || u.Path == "" || u.Path == "/" {
		t.Fatalf("invalid short url %q: %v", shortURL, err)
	}
	return u.Path
}

func assertRedirect(t *testing.T, path, wantLocation string) {
	t.Helper()
	resp, _ := do(t, http.MethodGet, path, "", nil)
	if resp.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("GET %s status = %d, want %d", path, resp.StatusCode, http.StatusTemporaryRedirect)
	}
	if got := resp.Header.Get("Location"); got != wantLocation {
		t.Errorf("GET %s Location = %q, want %q", path, got, wantLocation)
	}
}

func TestPing(t *testing.T) {
	resp, _ := do(t, http.MethodGet, "/ping", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /ping status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestShortenAndExpand(t *testing.T) {
	original := uniqueURL(t)
	resp, body := do(t, http.MethodPost, "/", "text/plain", []byte(original))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST / status = %d, want %d, body %q", resp.StatusCode, http.StatusCreated, body)
	}
	assertRedirect(t, shortPath(t, body), original)
}

func TestShortenJSONAndExpand(t *testing.T) {
	original := uniqueURL(t)
	reqBody, err := json.Marshal(api.ShortenRequest{URL: original})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	resp, body := do(t, http.MethodPost, "/api/shorten", "application/json", reqBody)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /api/shorten status = %d, want %d, body %q", resp.StatusCode, http.StatusCreated, body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var res api.ShortenResponse
	if err := json.Unmarshal([]byte(body), &res); err != nil {
		t.Fatalf("unmarshal response %q: %v", body, err)
	}
	assertRedirect(t, shortPath(t, res.Result), original)
}

func TestShortenSameURLTwice(t *testing.T) {
	original := uniqueURL(t)
	resp, first := do(t, http.MethodPost, "/", "text/plain", []byte(original))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST / #1 status = %d, want %d, body %q", resp.StatusCode, http.StatusCreated, first)
	}
	resp, second := do(t, http.MethodPost, "/", "text/plain", []byte(original))
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("POST / #2 status = %d, want %d, body %q", resp.StatusCode, http.StatusConflict, second)
	}
	if second != first {
		t.Errorf("POST / #2 short url = %q, want %q", second, first)
	}
	assertRedirect(t, shortPath(t, second), original)
}

func TestShortenJSONSameURLTwice(t *testing.T) {
	original := uniqueURL(t)
	first := shortenJSON(t, original, http.StatusCreated)
	second := shortenJSON(t, original, http.StatusConflict)
	if second != first {
		t.Errorf("POST /api/shorten #2 short url = %q, want %q", second, first)
	}
	assertRedirect(t, shortPath(t, second), original)
}

func TestShortenBatchAndExpand(t *testing.T) {
	base := uniqueURL(t)
	items := []api.ShortenBatchRequestItem{
		{CorrelationID: "a1", OriginalURL: base + "/a"},
		{CorrelationID: "b2", OriginalURL: base + "/b"},
	}
	res := shortenBatch(t, items, http.StatusCreated)
	if res[0].ShortURL == res[1].ShortURL {
		t.Errorf("different urls got the same short url %q", res[0].ShortURL)
	}
	for i, item := range items {
		assertRedirect(t, shortPath(t, res[i].ShortURL), item.OriginalURL)
	}
}

func TestShortenBatchConflict(t *testing.T) {
	base := uniqueURL(t)
	existingURL := base + "/existing"
	resp, existing := do(t, http.MethodPost, "/", "text/plain", []byte(existingURL))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST / status = %d, want %d, body %q", resp.StatusCode, http.StatusCreated, existing)
	}

	t.Run("some urls already shortened", func(t *testing.T) {
		items := []api.ShortenBatchRequestItem{
			{CorrelationID: "a1", OriginalURL: existingURL},
			{CorrelationID: "b2", OriginalURL: base + "/new"},
		}
		res := shortenBatch(t, items, http.StatusConflict)
		if res[0].ShortURL != existing {
			t.Errorf("existing url short_url = %q, want %q", res[0].ShortURL, existing)
		}
		for i, item := range items {
			assertRedirect(t, shortPath(t, res[i].ShortURL), item.OriginalURL)
		}
	})

	t.Run("all urls already shortened", func(t *testing.T) {
		items := []api.ShortenBatchRequestItem{
			{CorrelationID: "a1", OriginalURL: existingURL},
		}
		res := shortenBatch(t, items, http.StatusConflict)
		if res[0].ShortURL != existing {
			t.Errorf("existing url short_url = %q, want %q", res[0].ShortURL, existing)
		}
	})
}

func TestShortenBatchSameURLTwice(t *testing.T) {
	original := uniqueURL(t)
	items := []api.ShortenBatchRequestItem{
		{CorrelationID: "a1", OriginalURL: original},
		{CorrelationID: "b2", OriginalURL: original},
	}
	res := shortenBatch(t, items, http.StatusCreated)
	if res[0].ShortURL != res[1].ShortURL {
		t.Errorf("same url got different short urls %q and %q", res[0].ShortURL, res[1].ShortURL)
	}
	assertRedirect(t, shortPath(t, res[0].ShortURL), original)
}

func shortenJSON(t *testing.T, original string, wantStatus int) string {
	t.Helper()
	reqBody, err := json.Marshal(api.ShortenRequest{URL: original})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	resp, body := do(t, http.MethodPost, "/api/shorten", "application/json", reqBody)
	if resp.StatusCode != wantStatus {
		t.Fatalf("POST /api/shorten status = %d, want %d, body %q", resp.StatusCode, wantStatus, body)
	}
	var res api.ShortenResponse
	if err := json.Unmarshal([]byte(body), &res); err != nil {
		t.Fatalf("unmarshal response %q: %v", body, err)
	}
	return res.Result
}

func shortenBatch(t *testing.T, items []api.ShortenBatchRequestItem, wantStatus int) []api.ShortenBatchResponseItem {
	t.Helper()
	reqBody, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	resp, body := do(t, http.MethodPost, "/api/shorten/batch", "application/json", reqBody)
	if resp.StatusCode != wantStatus {
		t.Fatalf("POST /api/shorten/batch status = %d, want %d, body %q", resp.StatusCode, wantStatus, body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var res []api.ShortenBatchResponseItem
	if err := json.Unmarshal([]byte(body), &res); err != nil {
		t.Fatalf("unmarshal response %q: %v", body, err)
	}
	if len(res) != len(items) {
		t.Fatalf("results = %d, want %d", len(res), len(items))
	}
	for i, item := range items {
		if res[i].CorrelationID != item.CorrelationID {
			t.Errorf("result[%d] correlation_id = %q, want %q", i, res[i].CorrelationID, item.CorrelationID)
		}
	}
	return res
}

func TestExpandUnknownID(t *testing.T) {
	resp, _ := do(t, http.MethodGet, "/doesNotExist", "", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET unknown id status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestBadRequests(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		contentType string
		body        string
	}{
		{name: "empty body", path: "/", contentType: "text/plain", body: ""},
		{name: "json wrong content type", path: "/api/shorten", contentType: "text/plain", body: `{"url":"https://ya.ru"}`},
		{name: "json invalid body", path: "/api/shorten", contentType: "application/json", body: `{"url":`},
		{name: "json empty url", path: "/api/shorten", contentType: "application/json", body: `{"url":""}`},
		{name: "batch wrong content type", path: "/api/shorten/batch", contentType: "text/plain",
			body: `[{"correlation_id":"1","original_url":"https://ya.ru"}]`},
		{name: "batch invalid body", path: "/api/shorten/batch", contentType: "application/json", body: `[{"correlation_id":`},
		{name: "batch empty", path: "/api/shorten/batch", contentType: "application/json", body: `[]`},
		{name: "batch empty url", path: "/api/shorten/batch", contentType: "application/json",
			body: `[{"correlation_id":"1","original_url":""}]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := do(t, http.MethodPost, tt.path, tt.contentType, []byte(tt.body))
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d, body %q", resp.StatusCode, http.StatusBadRequest, body)
			}
		})
	}
}
