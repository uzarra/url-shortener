package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

const arrayEnd = "\n]\n"

type FileStorage struct {
	mem  *Storage
	path string

	mu    sync.Mutex
	count int
	end   int64
}

func NewFileStorage(path string) (*FileStorage, error) {
	s := &FileStorage{
		path: path,
		mem:  NewStorage(),
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
	f, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open storage file: %w", err)
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	tok, err := dec.Token()
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read storage file: %w", err)
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '[' {
		return errors.New("storage file is not a JSON array")
	}
	for dec.More() {
		var rec record
		if err := dec.Decode(&rec); err != nil {
			return fmt.Errorf("decode storage record: %w", err)
		}
		if err := s.mem.Save(rec.ShortURL, rec.OriginalURL); err != nil {
			return fmt.Errorf("restore record %s: %w", rec.ShortURL, err)
		}
		s.count++
		s.end = dec.InputOffset()
	}
	if _, err := dec.Token(); err != nil {
		return fmt.Errorf("read end of storage array: %w", err)
	}
	return nil
}

func (s *FileStorage) Save(id string, originalURL string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.mem.Load(id); err == nil {
		return ErrAlreadyExists
	}
	rec := record{
		UUID:        strconv.Itoa(s.count + 1),
		ShortURL:    id,
		OriginalURL: originalURL,
	}
	if err := s.appendRecord(rec); err != nil {
		return err
	}
	s.count++
	return s.mem.Save(id, originalURL)
}

func (s *FileStorage) appendRecord(rec record) error {
	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal record: %w", err)
	}
	offset, prefix := s.end, ",\n  "
	if s.count == 0 {
		offset, prefix = 0, "[\n  "
	}
	buf := make([]byte, 0, len(prefix)+len(data)+len(arrayEnd))
	buf = append(buf, prefix...)
	buf = append(buf, data...)
	buf = append(buf, arrayEnd...)

	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("open storage file: %w", err)
	}
	if _, err := f.WriteAt(buf, offset); err != nil {
		f.Close()
		return fmt.Errorf("write storage file: %w", err)
	}
	newSize := offset + int64(len(buf))
	if err := f.Truncate(newSize); err != nil {
		f.Close()
		return fmt.Errorf("truncate storage file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close storage file: %w", err)
	}
	s.end = newSize - int64(len(arrayEnd))
	return nil
}

func (s *FileStorage) Load(id string) (string, error) {
	return s.mem.Load(id)
}
