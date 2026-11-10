package auth

import (
	"errors"
	"time"
)

const TokenTTL = 30 * 24 * time.Hour

var (
	ErrInvalidToken      = errors.New("invalid token")
	ErrNoUserID          = errors.New("no user ID")
	ErrUserIDNotFound    = errors.New("user ID not found in context")
	ErrUserIDInvalidType = errors.New("user ID in context has unexpected type")
	ErrEmptyUserID       = errors.New("user ID in context is empty")
)

type Manager struct {
	secret []byte
	ttl    time.Duration
}

func NewManager(secret []byte, ttl time.Duration) *Manager {
	return &Manager{
		secret: secret,
		ttl:    ttl,
	}
}
