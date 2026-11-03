package auth

import (
	"errors"
	"time"
)

const TokenTTL = 30 * 24 * time.Hour

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrNoUserID     = errors.New("no user ID")
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
