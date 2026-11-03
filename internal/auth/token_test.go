package auth_test

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uzarra/url-shortener/internal/auth"
)

var testSecret = []byte("test-secret")

func newTestManager() *auth.Manager {
	return auth.NewManager(testSecret, time.Hour)
}

func signRaw(t *testing.T, method jwt.SigningMethod, key any, claims jwt.Claims) string {
	t.Helper()
	token, err := jwt.NewWithClaims(method, claims).SignedString(key)
	require.NoError(t, err)
	return token
}

func tamper(token string) string {
	b := []byte(token)
	i := len(b) - 10
	if b[i] == 'A' {
		b[i] = 'B'
	} else {
		b[i] = 'A'
	}
	return string(b)
}

func validClaims(userID string) auth.Claims {
	return auth.Claims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		UserID:    userID,
	}
}

func TestManager_BuildParse(t *testing.T) {
	m := newTestManager()
	userID := auth.NewUserID()

	token, err := m.Build(userID)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	got, err := m.Parse(token)
	require.NoError(t, err)
	assert.Equal(t, userID, got)
}

func TestManager_Parse_WrongSecret(t *testing.T) {
	token, err := auth.NewManager([]byte("other-secret"), time.Hour).Build(auth.NewUserID())
	require.NoError(t, err)

	_, err = newTestManager().Parse(token)
	assert.ErrorIs(t, err, auth.ErrInvalidToken)
	assert.ErrorIs(t, err, jwt.ErrTokenSignatureInvalid)
}

func TestManager_Parse_Malformed(t *testing.T) {
	m := newTestManager()
	valid, err := m.Build(auth.NewUserID())
	require.NoError(t, err)

	tests := []struct {
		name  string
		token string
	}{
		{name: "empty", token: ""},
		{name: "garbage", token: "not-a-jwt"},
		{name: "only dots", token: ".."},
		{name: "tampered signature", token: tamper(valid)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := m.Parse(tt.token)
			assert.ErrorIs(t, err, auth.ErrInvalidToken)
		})
	}
}

func TestManager_Parse_AlgNone(t *testing.T) {
	token := signRaw(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, validClaims(auth.NewUserID()))

	_, err := newTestManager().Parse(token)
	assert.ErrorIs(t, err, auth.ErrInvalidToken)
}

func TestManager_Parse_OtherHMACMethod(t *testing.T) {
	token := signRaw(t, jwt.SigningMethodHS512, testSecret, validClaims(auth.NewUserID()))
	_, err := newTestManager().Parse(token)
	assert.ErrorIs(t, err, auth.ErrInvalidToken)
	assert.ErrorIs(t, err, jwt.ErrTokenSignatureInvalid)
}

func TestManager_Parse_Expired(t *testing.T) {
	token, err := auth.NewManager(testSecret, -time.Minute).Build(auth.NewUserID())
	require.NoError(t, err)
	_, err = newTestManager().Parse(token)
	assert.ErrorIs(t, err, auth.ErrInvalidToken)
	assert.ErrorIs(t, err, jwt.ErrTokenExpired)
}

func TestManager_Parse_NoExpiration(t *testing.T) {
	token := signRaw(t, jwt.SigningMethodHS256, testSecret, auth.Claims{UserID: auth.NewUserID()})
	_, err := newTestManager().Parse(token)
	assert.ErrorIs(t, err, auth.ErrInvalidToken)
	assert.ErrorIs(t, err, jwt.ErrTokenRequiredClaimMissing)
}

func TestManager_Parse_BadUserID(t *testing.T) {
	m := newTestManager()
	tests := []struct {
		name   string
		userID string
	}{
		{name: "empty", userID: ""},
		{name: "not uuid", userID: "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := m.Build(tt.userID)
			require.NoError(t, err)

			_, err = m.Parse(token)
			assert.ErrorIs(t, err, auth.ErrNoUserID)
			assert.NotErrorIs(t, err, auth.ErrInvalidToken)
		})
	}
}
