package auth

import (
	"fmt"
	"time"
	"uuid"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	jwt.RegisteredClaims
	UserID string `json:"user_id"`
}

func (m *Manager) Build(userID string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.ttl)),
		UserID:    userID,
	})

	tokenString, err := token.SignedString(m.secret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func (m *Manager) Parse(tokenString string) (string, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		return m.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	if !token.Valid {
		return "", ErrInvalidToken
	}
	id, err := uuid.Parse(claims.UserID)
	if err != nil {
		return "", ErrNoUserID
	}
	return id.String(), nil
}
