package repository

import (
	"errors"
	"sync"
)

var ErrNotFound = errors.New("url not found")

type Storage struct {
	mu   sync.RWMutex
	urls map[string]string
}

func NewStorage() *Storage {
	return &Storage{
		urls: make(map[string]string),
	}
}

func (s *Storage) Save(id string, originalUrl string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.urls[id] = originalUrl
	return nil
}

func (s *Storage) Load(id string) (originalUrl string, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	originalUrl, ok := s.urls[id]
	if !ok {
		return "", ErrNotFound
	}
	return originalUrl, nil
}
