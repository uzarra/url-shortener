//go:build e2e

package e2e

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/uzarra/url-shortener/internal/api"
	"github.com/uzarra/url-shortener/internal/auth"
)

const deleteWait = 5 * time.Second

func shortenAs(t *testing.T, c *http.Client, original string) string {
	t.Helper()
	resp, body := doAs(t, c, http.MethodPost, "/", "text/plain", original)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST / %q status = %d, body %q", original, resp.StatusCode, body)
	}
	return strings.TrimPrefix(shortPath(t, body), "/")
}

func deleteAs(t *testing.T, c *http.Client, ids ...string) (*http.Response, string) {
	t.Helper()
	body, err := json.Marshal(ids)
	if err != nil {
		t.Fatalf("marshal ids: %v", err)
	}
	return doAs(t, c, http.MethodDelete, "/api/user/urls", "application/json", string(body))
}

func mustDeleteAccepted(t *testing.T, c *http.Client, ids ...string) {
	t.Helper()
	resp, body := deleteAs(t, c, ids...)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("DELETE /api/user/urls %v status = %d, want %d, body %q", ids, resp.StatusCode, http.StatusAccepted, body)
	}
}

func expandStatus(t *testing.T, id string) int {
	t.Helper()
	resp, _ := do(t, http.MethodGet, "/"+id, "", nil)
	return resp.StatusCode
}

func waitGone(t *testing.T, id string) {
	t.Helper()
	deadline := time.Now().Add(deleteWait)
	last := 0
	for time.Now().Before(deadline) {
		if last = expandStatus(t, id); last == http.StatusGone {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("GET /%s status = %d after %s, want %d", id, last, deleteWait, http.StatusGone)
}

func TestDeleteURLs_AcceptedThenGone(t *testing.T) {
	alice := newUserClient(t)
	base := uniqueURL(t)
	plain := shortenAs(t, alice, base+"/plain")

	batchBody, _ := json.Marshal([]api.ShortenBatchRequestItem{{CorrelationID: "1", OriginalURL: base + "/batch"}})
	resp, body := doAs(t, alice, http.MethodPost, "/api/shorten/batch", "application/json", string(batchBody))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /api/shorten/batch status = %d, body %q", resp.StatusCode, body)
	}
	var batchRes []api.ShortenBatchResponseItem
	if err := json.Unmarshal([]byte(body), &batchRes); err != nil {
		t.Fatalf("unmarshal %q: %v", body, err)
	}
	batch := strings.TrimPrefix(shortPath(t, batchRes[0].ShortURL), "/")

	mustDeleteAccepted(t, alice, plain, batch)

	waitGone(t, plain)
	waitGone(t, batch)
}

func TestDeleteURLs_OnlyOwnerCanDelete(t *testing.T) {
	alice, bob := newUserClient(t), newUserClient(t)
	base := uniqueURL(t)
	aliceURL := base + "/alice"
	aliceID := shortenAs(t, alice, aliceURL)
	bobMarker := shortenAs(t, bob, base+"/bob-marker")

	mustDeleteAccepted(t, bob, aliceID, bobMarker)
	waitGone(t, bobMarker)

	assertRedirect(t, "/"+aliceID, aliceURL)
}

func TestDeleteURLs_RemovedFromUserList(t *testing.T) {
	alice := newUserClient(t)
	base := uniqueURL(t)
	delID := shortenAs(t, alice, base+"/del")
	keepURL := base + "/keep"
	keepID := shortenAs(t, alice, keepURL)

	mustDeleteAccepted(t, alice, delID)
	waitGone(t, delID)

	resp, items := getUserURLs(t, alice)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/user/urls status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if len(items) != 1 || items[0].OriginalURL != keepURL {
		t.Fatalf("user urls = %+v, want only %q", items, keepURL)
	}

	mustDeleteAccepted(t, alice, keepID)
	waitGone(t, keepID)

	resp, _ = getUserURLs(t, alice)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("GET /api/user/urls after deleting all status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}

func TestDeleteURLs_UnknownAndRepeatedIDs(t *testing.T) {
	alice := newUserClient(t)
	id := shortenAs(t, alice, uniqueURL(t))
	unknown := fmt.Sprintf("nx%d", time.Now().UnixNano())

	mustDeleteAccepted(t, alice, unknown, id, id)
	waitGone(t, id)

	mustDeleteAccepted(t, alice, id)
	if got := expandStatus(t, id); got != http.StatusGone {
		t.Errorf("GET deleted id after repeated delete status = %d, want %d", got, http.StatusGone)
	}
	if got := expandStatus(t, unknown); got != http.StatusNotFound {
		t.Errorf("GET unknown id status = %d, want %d", got, http.StatusNotFound)
	}
}

func TestDeleteURLs_Gzip(t *testing.T) {
	alice := newUserClient(t)
	id := shortenAs(t, alice, uniqueURL(t))

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, _ = fmt.Fprintf(zw, `[%q]`, id)
	_ = zw.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodDelete, baseURL+"/api/user/urls", &buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	resp, err := alice.Do(req)
	if err != nil {
		t.Fatalf("DELETE gzip: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("DELETE gzip status = %d, want %d", resp.StatusCode, http.StatusAccepted)
	}
	waitGone(t, id)
}

func TestDeleteURLs_BadRequests(t *testing.T) {
	alice := newUserClient(t)
	id := shortenAs(t, alice, uniqueURL(t))

	tests := []struct {
		name        string
		contentType string
		body        string
	}{
		{name: "wrong content-type", contentType: "text/plain", body: `["` + id + `"]`},
		{name: "invalid json", contentType: "application/json", body: `not json`},
		{name: "object instead of array", contentType: "application/json", body: `{"id":"` + id + `"}`},
		{name: "empty array", contentType: "application/json", body: `[]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := doAs(t, alice, http.MethodDelete, "/api/user/urls", tt.contentType, tt.body)
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("status = %d, want %d, body %q", resp.StatusCode, http.StatusBadRequest, body)
			}
		})
	}
	if got := expandStatus(t, id); got != http.StatusTemporaryRedirect {
		t.Errorf("GET /%s status = %d after bad requests, want %d", id, got, http.StatusTemporaryRedirect)
	}
}

func TestDeleteURLs_CookieWithoutUserIDUnauthorized(t *testing.T) {
	secret := os.Getenv("E2E_SECRET_KEY")
	if secret == "" {
		t.Skip("E2E_SECRET_KEY is not set")
	}
	token, err := auth.NewManager([]byte(secret), time.Hour).Build("")
	if err != nil {
		t.Fatalf("build token: %v", err)
	}
	c := newUserClient(t)
	u, _ := url.Parse(baseURL)
	c.Jar.SetCookies(u, []*http.Cookie{{Name: authCookieName, Value: token, Path: "/"}})

	resp, body := deleteAs(t, c, "anything")
	if authCookie(resp) != nil {
		t.Fatalf("server rejected the token signature and issued a new cookie: " +
			"E2E_SECRET_KEY must match the server's SECRET_KEY (default \"dev-secret\")")
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d, body %q", resp.StatusCode, http.StatusUnauthorized, body)
	}
}

func TestDeleteURLs_ConcurrentUsers(t *testing.T) {
	const users, perUser = 10, 5
	base := uniqueURL(t)

	clients := make([]*http.Client, users)
	ids := make([][]string, users)
	keep := make([]string, users)
	for u := range users {
		clients[u] = newUserClient(t)
		for i := range perUser {
			ids[u] = append(ids[u], shortenAs(t, clients[u], fmt.Sprintf("%s/u%d/%d", base, u, i)))
		}
		keep[u] = fmt.Sprintf("%s/u%d/keep", base, u)
		shortenAs(t, clients[u], keep[u])
	}

	statuses := make([]int, users)
	errs := make([]error, users)
	var wg sync.WaitGroup
	for u := range users {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body, _ := json.Marshal(ids[u])
			req, err := http.NewRequestWithContext(t.Context(), http.MethodDelete, baseURL+"/api/user/urls", bytes.NewReader(body))
			if err != nil {
				errs[u] = err
				return
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := clients[u].Do(req)
			if err != nil {
				errs[u] = err
				return
			}
			resp.Body.Close()
			statuses[u] = resp.StatusCode
		}()
	}
	wg.Wait()

	for u := range users {
		if errs[u] != nil {
			t.Fatalf("user %d DELETE: %v", u, errs[u])
		}
		if statuses[u] != http.StatusAccepted {
			t.Fatalf("user %d DELETE status = %d, want %d", u, statuses[u], http.StatusAccepted)
		}
	}
	for u := range users {
		for _, id := range ids[u] {
			waitGone(t, id)
		}
		_, items := getUserURLs(t, clients[u])
		if len(items) != 1 || items[0].OriginalURL != keep[u] {
			t.Errorf("user %d urls = %+v, want only %q", u, items, keep[u])
		}
	}
}
