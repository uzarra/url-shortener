package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/uzarra/url-shortener/internal/model"
)

type record struct {
	UUID        string `json:"uuid"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
	UserID      string `json:"user_id,omitempty"`
	IsDeleted   bool   `json:"is_deleted,omitempty"`
}

const arrayEnd = "\n]\n"

type FileStorage struct {
	mem  *Storage
	path string

	mu    sync.Mutex
	count int
	end   int64
	recs  []record
	index map[string]int
}

func NewFileStorage(path string) (*FileStorage, error) {
	s := &FileStorage{
		path:  path,
		mem:   NewStorage(),
		index: make(map[string]int),
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create storage dir: %w", err)
	}
	if err := s.restore(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *FileStorage) remember(rec record) {
	s.index[rec.ShortURL] = len(s.recs)
	s.recs = append(s.recs, rec)
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
		if decodeErr := dec.Decode(&rec); decodeErr != nil {
			return fmt.Errorf("decode storage record: %w", decodeErr)
		}
		if saveErr := s.mem.Save(context.Background(), model.URL{
			ShortID:     rec.ShortURL,
			OriginalURL: rec.OriginalURL,
			UserID:      rec.UserID,
			DeletedFlag: rec.IsDeleted,
		}); saveErr != nil {
			return fmt.Errorf("restore record %s: %w", rec.ShortURL, saveErr)
		}
		s.remember(rec)
		s.count++
		s.end = dec.InputOffset()
	}
	if _, err := dec.Token(); err != nil {
		return fmt.Errorf("read end of storage array: %w", err)
	}
	return nil
}

func (s *FileStorage) Save(ctx context.Context, url model.URL) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.mem.Load(ctx, url.ShortID); !errors.Is(err, ErrNotFound) {
		return ErrAlreadyExists
	}
	if _, err := s.mem.GetShortURLByOriginalURL(ctx, url.OriginalURL); err == nil {
		return ErrConflict
	}
	rec := record{
		UUID:        strconv.Itoa(s.count + 1),
		ShortURL:    url.ShortID,
		OriginalURL: url.OriginalURL,
		UserID:      url.UserID,
	}
	if err := s.appendRecords([]record{rec}); err != nil {
		return err
	}
	s.remember(rec)
	s.count++
	return s.mem.Save(ctx, url)
}

func (s *FileStorage) Load(ctx context.Context, id string) (string, error) {
	return s.mem.Load(ctx, id)
}

func (s *FileStorage) SaveBatch(ctx context.Context, urls []model.URL) error {
	if len(urls) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := make(map[string]struct{}, len(urls))
	seenOriginal := make(map[string]struct{}, len(urls))
	recs := make([]record, 0, len(urls))
	for i, u := range urls {
		if _, err := s.mem.Load(ctx, u.ShortID); !errors.Is(err, ErrNotFound) {
			return ErrAlreadyExists
		}
		if _, err := s.mem.GetShortURLByOriginalURL(ctx, u.OriginalURL); err == nil {
			return ErrConflict
		}
		if _, ok := seen[u.ShortID]; ok {
			return ErrAlreadyExists
		}
		if _, ok := seenOriginal[u.OriginalURL]; ok {
			return ErrConflict
		}
		seen[u.ShortID] = struct{}{}
		seenOriginal[u.OriginalURL] = struct{}{}
		recs = append(recs, record{
			UUID:        strconv.Itoa(s.count + i + 1),
			ShortURL:    u.ShortID,
			OriginalURL: u.OriginalURL,
			UserID:      u.UserID,
		})
	}
	if err := s.appendRecords(recs); err != nil {
		return err
	}
	for _, rec := range recs {
		s.remember(rec)
	}
	s.count += len(recs)
	return s.mem.SaveBatch(ctx, urls)
}

func (s *FileStorage) GetShortURLByOriginalURL(ctx context.Context, originalURL string) (string, error) {
	return s.mem.GetShortURLByOriginalURL(ctx, originalURL)
}

func (s *FileStorage) GetURLsByUserID(ctx context.Context, userID string) ([]model.URL, error) {
	return s.mem.GetURLsByUserID(ctx, userID)
}

func (s *FileStorage) DeleteURLs(ctx context.Context, reqs []model.DeleteRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := make([]int, 0, len(reqs))
	for _, req := range reqs {
		i, ok := s.index[req.ShortID]
		if !ok || req.UserID == "" || s.recs[i].UserID != req.UserID || s.recs[i].IsDeleted {
			continue
		}
		s.recs[i].IsDeleted = true
		changed = append(changed, i)
	}
	if len(changed) == 0 {
		return nil
	}
	if err := s.rewrite(); err != nil {
		for _, i := range changed {
			s.recs[i].IsDeleted = false
		}
		return err
	}
	return s.mem.DeleteURLs(ctx, reqs)
}

func (s *FileStorage) rewrite() error {
	var buf []byte
	for i, rec := range s.recs {
		data, err := json.Marshal(rec)
		if err != nil {
			return fmt.Errorf("marshal record: %w", err)
		}
		prefix := ",\n  "
		if i == 0 {
			prefix = "[\n  "
		}
		buf = append(buf, prefix...)
		buf = append(buf, data...)
	}
	buf = append(buf, arrayEnd...)

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, buf, 0o644); err != nil {
		return fmt.Errorf("write storage file: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("replace storage file: %w", err)
	}
	s.end = int64(len(buf) - len(arrayEnd))
	return nil
}

func (s *FileStorage) appendRecords(recs []record) error {
	offset := s.end
	if s.count == 0 {
		offset = 0
	}
	var buf []byte
	for i, rec := range recs {
		data, err := json.Marshal(rec)
		if err != nil {
			return fmt.Errorf("marshal record: %w", err)
		}
		prefix := ",\n  "
		if s.count == 0 && i == 0 {
			prefix = "[\n  "
		}
		buf = append(buf, prefix...)
		buf = append(buf, data...)
	}
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
