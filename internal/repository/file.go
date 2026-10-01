package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

type record struct {
	UUID        string `json:"uuid"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

type FileStorage struct {
	mu      sync.RWMutex
	path    string
	urls    map[string]string
	records []record
}

func NewFileStorage(path string) (*FileStorage, error) {
	s := &FileStorage{
		path: path,
		urls: make(map[string]string),
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create storage dir: %w", err)
	}
	if err := s.restore(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *FileStorage) restore() error {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read storage file: %w", err)
	}
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, &s.records); err != nil {
		return fmt.Errorf("unmarshal storage file data: %w", err)
	}
	for _, rec := range s.records {
		s.urls[rec.ShortURL] = rec.OriginalURL
	}
	return nil
}

func (s *FileStorage) Save(id string, originalURL string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.urls[id]; ok {
		return ErrAlreadyExists
	}
	rec := record{
		UUID:        strconv.Itoa(len(s.records) + 1),
		ShortURL:    id,
		OriginalURL: originalURL,
	}
	records := append(s.records, rec)
	if err := s.flush(records); err != nil {
		return err
	}
	s.records = records
	s.urls[id] = originalURL
	return nil
}

func (s *FileStorage) flush(records []record) error {
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("encode storage file: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write storage: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("replace storage file: %w", err)
	}
	return nil
}

func (s *FileStorage) Load(id string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	originalURL, ok := s.urls[id]
	if !ok {
		return "", ErrNotFound
	}
	return originalURL, nil
}
