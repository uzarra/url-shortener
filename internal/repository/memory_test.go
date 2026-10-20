package repository

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/uzarra/url-shortener/internal/model"
)

func TestStorage_Load(t *testing.T) {
	s := NewStorage()
	if err := s.Save(t.Context(), "abc", "http://yandex.ru"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	tests := []struct {
		name    string
		id      string
		want    string
		wantErr error
	}{
		{name: "existing id", id: "abc", want: "http://yandex.ru"},
		{name: "missing id", id: "missing", wantErr: ErrNotFound},
		{name: "empty id", id: "", wantErr: ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.Load(t.Context(), tt.id)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Load() error = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Load() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStorage_SaveDuplicate(t *testing.T) {
	s := NewStorage()
	if err := s.Save(t.Context(), "abc", "http://yandex.ru"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := s.Save(t.Context(), "abc", "http://ya.ru"); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("Save() duplicate error = %v, want %v", err, ErrAlreadyExists)
	}
	if got, _ := s.Load(t.Context(), "abc"); got != "http://yandex.ru" {
		t.Errorf("Load() = %q, want original %q", got, "http://yandex.ru")
	}
}

func TestStorage_SaveKeepsSeparateIDs(t *testing.T) {
	s := NewStorage()
	urls := map[string]string{
		"4rSPg8ap": "http://yandex.ru",
		"edVPg3ks": "http://ya.ru",
	}
	for id, u := range urls {
		if err := s.Save(t.Context(), id, u); err != nil {
			t.Fatalf("Save(%q) error = %v", id, err)
		}
	}
	for id, want := range urls {
		if got, err := s.Load(t.Context(), id); err != nil || got != want {
			t.Errorf("Load(%q) = %q, %v, want %q, nil", id, got, err, want)
		}
	}
}

func TestStorage_Concurrent(t *testing.T) {
	s := NewStorage()
	const n = 50
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(2)
		id := fmt.Sprintf("id%d", i)
		go func() {
			defer wg.Done()
			if err := s.Save(t.Context(), id, "http://example.com"); err != nil {
				t.Errorf("Save(%q) error = %v", id, err)
			}
		}()
		go func() {
			defer wg.Done()
			_, _ = s.Load(t.Context(), id)
		}()
	}
	wg.Wait()
	for i := range n {
		id := fmt.Sprintf("id%d", i)
		if _, err := s.Load(t.Context(), id); err != nil {
			t.Errorf("Load(%q) error = %v", id, err)
		}
	}
}

func TestStorage_ConcurrentSameID(t *testing.T) {
	s := NewStorage()
	const n = 50
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		success int
	)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := s.Save(t.Context(), "same", fmt.Sprintf("http://example.com/%d", i))
			if err == nil {
				mu.Lock()
				success++
				mu.Unlock()
				return
			}
			if !errors.Is(err, ErrAlreadyExists) {
				t.Errorf("Save() error = %v, want nil or %v", err, ErrAlreadyExists)
			}
		}()
	}
	wg.Wait()
	if success != 1 {
		t.Errorf("successful saves = %d, want 1", success)
	}
}

func TestStorage_SaveBatch(t *testing.T) {
	s := NewStorage()
	urls := []model.URL{
		{ShortID: "4rSPg8ap", OriginalURL: "http://yandex.ru"},
		{ShortID: "edVPg3ks", OriginalURL: "http://ya.ru"},
	}
	if err := s.SaveBatch(t.Context(), urls); err != nil {
		t.Fatalf("SaveBatch() error = %v", err)
	}
	for _, u := range urls {
		if got, err := s.Load(t.Context(), u.ShortID); err != nil || got != u.OriginalURL {
			t.Errorf("Load(%q) = %q, %v, want %q, nil", u.ShortID, got, err, u.OriginalURL)
		}
	}
}

func TestStorage_SaveBatchDuplicate(t *testing.T) {
	tests := []struct {
		name string
		urls []model.URL
	}{
		{name: "id already stored", urls: []model.URL{
			{ShortID: "newID123", OriginalURL: "http://new.ru"},
			{ShortID: "abc", OriginalURL: "http://ya.ru"},
		}},
		{name: "duplicate id inside batch", urls: []model.URL{
			{ShortID: "newID123", OriginalURL: "http://new.ru"},
			{ShortID: "newID123", OriginalURL: "http://ya.ru"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStorage()
			if err := s.Save(t.Context(), "abc", "http://yandex.ru"); err != nil {
				t.Fatalf("Save() error = %v", err)
			}
			if err := s.SaveBatch(t.Context(), tt.urls); !errors.Is(err, ErrAlreadyExists) {
				t.Fatalf("SaveBatch() error = %v, want %v", err, ErrAlreadyExists)
			}
			if _, err := s.Load(t.Context(), "newID123"); !errors.Is(err, ErrNotFound) {
				t.Errorf("Load() after failed SaveBatch error = %v, want %v", err, ErrNotFound)
			}
			if got, _ := s.Load(t.Context(), "abc"); got != "http://yandex.ru" {
				t.Errorf("Load() = %q, want original %q", got, "http://yandex.ru")
			}
		})
	}
}

func TestStorage_SaveBatchConcurrentSameID(t *testing.T) {
	s := NewStorage()
	const n = 50
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		success int
	)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := s.SaveBatch(t.Context(), []model.URL{
				{ShortID: fmt.Sprintf("unique%d", i), OriginalURL: "http://example.com"},
				{ShortID: "same", OriginalURL: fmt.Sprintf("http://example.com/%d", i)},
			})
			if err == nil {
				mu.Lock()
				success++
				mu.Unlock()
				return
			}
			if !errors.Is(err, ErrAlreadyExists) {
				t.Errorf("SaveBatch() error = %v, want nil or %v", err, ErrAlreadyExists)
			}
		}()
	}
	wg.Wait()
	if success != 1 {
		t.Errorf("successful batches = %d, want 1", success)
	}
}
