//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/uzarra/url-shortener/internal/api"
	"github.com/uzarra/url-shortener/internal/auth"
)

const authCookieName = "url-shortener.token"

func newUserClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	return &http.Client{
		Jar:           jar,
		Timeout:       5 * time.Second,
		CheckRedirect: client.CheckRedirect,
	}
}

func doAs(t *testing.T, c *http.Client, method, path, contentType, body string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, baseURL+path, bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.Do(req)
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

func authCookie(resp *http.Response) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == authCookieName {
			return c
		}
	}
	return nil
}

func getUserURLs(t *testing.T, c *http.Client) (*http.Response, []api.UserURLResponseItem) {
	t.Helper()
	resp, body := doAs(t, c, http.MethodGet, "/api/user/urls", "", "")
	if resp.StatusCode != http.StatusOK {
		return resp, nil
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("GET /api/user/urls Content-Type = %q, want application/json", ct)
	}
	var items []api.UserURLResponseItem
	if err := json.Unmarshal([]byte(body), &items); err != nil {
		t.Fatalf("unmarshal %q: %v", body, err)
	}
	return resp, items
}

func TestUserURLs_AllWaysOfShortening(t *testing.T) {
	alice := newUserClient(t)
	base := uniqueURL(t)
	want := map[string]string{} // original -> short

	resp, short := doAs(t, alice, http.MethodPost, "/", "text/plain", base+"/plain")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST / status = %d, body %q", resp.StatusCode, short)
	}
	if authCookie(resp) == nil {
		t.Fatal("first request must set auth cookie")
	}
	want[base+"/plain"] = short

	reqBody, _ := json.Marshal(api.ShortenRequest{URL: base + "/json"})
	resp, body := doAs(t, alice, http.MethodPost, "/api/shorten", "application/json", string(reqBody))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /api/shorten status = %d, body %q", resp.StatusCode, body)
	}
	if authCookie(resp) != nil {
		t.Error("valid cookie must not be reissued")
	}
	var shortenRes api.ShortenResponse
	if err := json.Unmarshal([]byte(body), &shortenRes); err != nil {
		t.Fatalf("unmarshal %q: %v", body, err)
	}
	want[base+"/json"] = shortenRes.Result

	batchBody, _ := json.Marshal([]api.ShortenBatchRequestItem{
		{CorrelationID: "1", OriginalURL: base + "/batch1"},
		{CorrelationID: "2", OriginalURL: base + "/batch2"},
	})
	resp, body = doAs(t, alice, http.MethodPost, "/api/shorten/batch", "application/json", string(batchBody))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /api/shorten/batch status = %d, body %q", resp.StatusCode, body)
	}
	var batchRes []api.ShortenBatchResponseItem
	if err := json.Unmarshal([]byte(body), &batchRes); err != nil {
		t.Fatalf("unmarshal %q: %v", body, err)
	}
	want[base+"/batch1"] = batchRes[0].ShortURL
	want[base+"/batch2"] = batchRes[1].ShortURL

	resp, items := getUserURLs(t, alice)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/user/urls status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	got := map[string]string{}
	for _, it := range items {
		got[it.OriginalURL] = it.ShortURL
	}
	if len(got) != len(want) {
		t.Fatalf("user urls = %v, want %v", got, want)
	}
	for original, short := range want {
		if got[original] != short {
			t.Errorf("short_url for %q = %q, want %q", original, got[original], short)
		}
		assertRedirect(t, shortPath(t, short), original)
	}
}

func TestUserURLs_IsolatedBetweenUsers(t *testing.T) {
	alice, bob := newUserClient(t), newUserClient(t)
	aliceURL, bobURL := uniqueURL(t)+"/alice", uniqueURL(t)+"/bob"

	if resp, body := doAs(t, alice, http.MethodPost, "/", "text/plain", aliceURL); resp.StatusCode != http.StatusCreated {
		t.Fatalf("alice POST / status = %d, body %q", resp.StatusCode, body)
	}
	if resp, body := doAs(t, bob, http.MethodPost, "/", "text/plain", bobURL); resp.StatusCode != http.StatusCreated {
		t.Fatalf("bob POST / status = %d, body %q", resp.StatusCode, body)
	}

	_, items := getUserURLs(t, bob)
	if len(items) != 1 || items[0].OriginalURL != bobURL {
		t.Fatalf("bob urls = %+v, want only %q", items, bobURL)
	}
}

func TestUserURLs_NewUserNoContent(t *testing.T) {
	c := newUserClient(t)

	resp, body := doAs(t, c, http.MethodGet, "/api/user/urls", "", "")

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d, body %q", resp.StatusCode, http.StatusNoContent, body)
	}
	if body != "" {
		t.Errorf("204 body = %q, want empty", body)
	}
	ck := authCookie(resp)
	if ck == nil {
		t.Fatal("new user must get auth cookie")
	}
	if ck.Path != "/" || !ck.HttpOnly || ck.MaxAge <= 0 {
		t.Errorf("cookie attrs: Path=%q HttpOnly=%v MaxAge=%d, want Path=/ HttpOnly=true MaxAge>0", ck.Path, ck.HttpOnly, ck.MaxAge)
	}
}

func TestUserURLs_TamperedCookieReplaced(t *testing.T) {
	alice := newUserClient(t)
	if resp, body := doAs(t, alice, http.MethodPost, "/", "text/plain", uniqueURL(t)); resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST / status = %d, body %q", resp.StatusCode, body)
	}

	u, _ := url.Parse(baseURL)
	cookies := alice.Jar.Cookies(u)
	var token string
	for _, ck := range cookies {
		if ck.Name == authCookieName {
			token = ck.Value
		}
	}
	if token == "" {
		t.Fatal("no auth cookie in jar")
	}
	tampered := token[:len(token)-10] + "AAAAAAAAAA"
	attacker := newUserClient(t)
	attacker.Jar.SetCookies(u, []*http.Cookie{{Name: authCookieName, Value: tampered, Path: "/"}})

	resp, _ := doAs(t, attacker, http.MethodGet, "/api/user/urls", "", "")
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("tampered cookie status = %d, want %d (new user, no urls)", resp.StatusCode, http.StatusNoContent)
	}
	if authCookie(resp) == nil {
		t.Error("tampered cookie must be replaced with a new one")
	}
}

func TestUserURLs_CookieWithoutUserIDUnauthorized(t *testing.T) {
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

	resp, body := doAs(t, c, http.MethodGet, "/api/user/urls", "", "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d, body %q", resp.StatusCode, http.StatusUnauthorized, body)
	}
}
