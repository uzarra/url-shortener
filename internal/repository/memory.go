package repository

import (
	"context"
	"errors"
	"sync"

	"github.com/uzarra/url-shortener/internal/model"
)

var (
	ErrNotFound      = errors.New("url not found")
	ErrAlreadyExists = errors.New("id already exists")
	ErrConflict      = errors.New("url conflict")
)

type Storage struct {
	mu         sync.RWMutex
	urls       map[string]string
	byOriginal map[string]string
	byUser     map[string][]string
}

func NewStorage() *Storage {
	return &Storage{
		urls:       make(map[string]string),
		byOriginal: make(map[string]string),
		byUser:     make(map[string][]string),
	}
}

func (s *Storage) Save(_ context.Context, url model.URL) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.urls[url.ShortID]; ok {
		return ErrAlreadyExists
	}
	if _, ok := s.byOriginal[url.OriginalURL]; ok {
		return ErrConflict
	}
	s.urls[url.ShortID] = url.OriginalURL
	s.byOriginal[url.OriginalURL] = url.ShortID
	if url.UserID != "" {
		s.byUser[url.UserID] = append(s.byUser[url.UserID], url.ShortID)
	}
	return nil
}

func (s *Storage) Load(_ context.Context, id string) (originalURL string, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	originalURL, ok := s.urls[id]
	if !ok {
		return "", ErrNotFound
	}
	return originalURL, nil
}

func (s *Storage) SaveBatch(_ context.Context, urls []model.URL) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := make(map[string]struct{}, len(urls))
	seenOriginal := make(map[string]struct{}, len(urls))
	for _, u := range urls {
		if _, ok := s.urls[u.ShortID]; ok {
			return ErrAlreadyExists
		}
		if _, ok := seen[u.ShortID]; ok {
			return ErrAlreadyExists
		}
		if _, ok := s.byOriginal[u.OriginalURL]; ok {
			return ErrConflict
		}
		if _, ok := seenOriginal[u.OriginalURL]; ok {
			return ErrConflict
		}
		seen[u.ShortID] = struct{}{}
		seenOriginal[u.OriginalURL] = struct{}{}
	}
	for _, u := range urls {
		s.urls[u.ShortID] = u.OriginalURL
		s.byOriginal[u.OriginalURL] = u.ShortID
		if u.UserID != "" {
			s.byUser[u.UserID] = append(s.byUser[u.UserID], u.ShortID)
		}
	}
	return nil
}

func (s *Storage) GetShortURLByOriginalURL(_ context.Context, originalURL string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	shortID, ok := s.byOriginal[originalURL]
	if !ok {
		return "", ErrNotFound
	}
	return shortID, nil
}

func (s *Storage) GetURLsByUserID(_ context.Context, userID string) ([]model.URL, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.byUser[userID]
	result := make([]model.URL, 0, len(ids))
	for _, id := range ids {
		result = append(result, model.URL{ShortID: id, OriginalURL: s.urls[id], UserID: userID})
	}
	return result, nil
}
