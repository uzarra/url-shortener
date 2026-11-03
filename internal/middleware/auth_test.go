package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uzarra/url-shortener/internal/auth"
)

var authTestKey = []byte("middleware-test-key")

type authResult struct {
	rec    *httptest.ResponseRecorder
	called bool
	userID string
	hasID  bool
}

func serveAuth(t *testing.T, m *auth.Manager, cookie *http.Cookie) authResult {
	t.Helper()
	var res authResult
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res.called = true
		res.userID, res.hasID = auth.UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusTeapot)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/shorten/batch", nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	res.rec = httptest.NewRecorder()
	Auth(m)(next).ServeHTTP(res.rec, req)
	return res
}

func issuedCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == cookieName {
			return c
		}
	}
	return nil
}

func validCookie(t *testing.T, m *auth.Manager) (*http.Cookie, string) {
	t.Helper()
	userID := auth.NewUserID()
	token, err := m.Build(userID)
	require.NoError(t, err)
	return &http.Cookie{Name: cookieName, Value: token}, userID
}

func TestAuth_NoCookie_IssuesNew(t *testing.T) {
	m := auth.NewManager(authTestKey, time.Hour)

	res := serveAuth(t, m, nil)

	require.True(t, res.called)
	assert.Equal(t, http.StatusTeapot, res.rec.Code)
	require.True(t, res.hasID, "user id must be in context")

	c := issuedCookie(t, res.rec)
	require.NotNil(t, c, "Set-Cookie must be sent")
	gotID, err := m.Parse(c.Value)
	require.NoError(t, err)
	assert.Equal(t, res.userID, gotID, "id in context must match id in cookie")
}

func TestAuth_CookieAttributes(t *testing.T) {
	res := serveAuth(t, auth.NewManager(authTestKey, time.Hour), nil)

	c := issuedCookie(t, res.rec)
	require.NotNil(t, c)
	assert.Equal(t, "/", c.Path)
	assert.True(t, c.HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
	assert.Equal(t, int(auth.TokenTTL.Seconds()), c.MaxAge)
}

func TestAuth_ValidCookie_KeepsUser(t *testing.T) {
	m := auth.NewManager(authTestKey, time.Hour)
	cookie, userID := validCookie(t, m)

	res := serveAuth(t, m, cookie)

	require.True(t, res.called)
	require.True(t, res.hasID)
	assert.Equal(t, userID, res.userID)
	assert.Nil(t, issuedCookie(t, res.rec), "valid cookie must not be reissued")
}

func TestAuth_InvalidCookie_IssuesNew(t *testing.T) {
	m := auth.NewManager(authTestKey, time.Hour)
	validToken, err := m.Build(auth.NewUserID())
	require.NoError(t, err)
	expiredToken, err := auth.NewManager(authTestKey, -time.Minute).Build(auth.NewUserID())
	require.NoError(t, err)
	foreignToken, err := auth.NewManager([]byte("another-key"), time.Hour).Build(auth.NewUserID())
	require.NoError(t, err)

	tests := []struct {
		name  string
		value string
	}{
		{name: "garbage", value: "not-a-jwt"},
		{name: "tampered", value: validToken[:len(validToken)-10] + "AAAAAAAAAA"},
		{name: "expired", value: expiredToken},
		{name: "signed with another key", value: foreignToken},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := serveAuth(t, m, &http.Cookie{Name: cookieName, Value: tt.value})

			require.True(t, res.called)
			require.True(t, res.hasID)

			c := issuedCookie(t, res.rec)
			require.NotNil(t, c, "invalid cookie must be replaced")
			gotID, err := m.Parse(c.Value)
			require.NoError(t, err)
			assert.Equal(t, res.userID, gotID)
		})
	}
}

func TestAuth_CookieWithoutUserID_NoNewCookie(t *testing.T) {
	m := auth.NewManager(authTestKey, time.Hour)
	token, err := m.Build("")
	require.NoError(t, err)

	res := serveAuth(t, m, &http.Cookie{Name: cookieName, Value: token})

	require.True(t, res.called, "next must be called, handler decides on 401")
	assert.False(t, res.hasID, "no user id must be in context")
	assert.Nil(t, issuedCookie(t, res.rec), "signed cookie without id must not be reissued")
}

func TestAuth_OtherCookieIgnored(t *testing.T) {
	m := auth.NewManager(authTestKey, time.Hour)
	token, err := m.Build(auth.NewUserID())
	require.NoError(t, err)

	res := serveAuth(t, m, &http.Cookie{Name: "some-other-cookie", Value: token})

	require.True(t, res.hasID)
	assert.NotNil(t, issuedCookie(t, res.rec), "token under another name must not be accepted")
}

func TestAuth_NewUsersGetDifferentIDs(t *testing.T) {
	m := auth.NewManager(authTestKey, time.Hour)

	first := serveAuth(t, m, nil)
	second := serveAuth(t, m, nil)

	assert.NotEqual(t, first.userID, second.userID)
}
