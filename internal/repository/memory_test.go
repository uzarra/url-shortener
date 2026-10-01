package repository

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestStorage_Load(t *testing.T) {
	s := NewStorage()
	if err := s.Save("abc", "http://yandex.ru"); err != nil {
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
			got, err := s.Load(tt.id)
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
	if err := s.Save("abc", "http://yandex.ru"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := s.Save("abc", "http://ya.ru"); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("Save() duplicate error = %v, want %v", err, ErrAlreadyExists)
	}
	if got, _ := s.Load("abc"); got != "http://yandex.ru" {
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
		if err := s.Save(id, u); err != nil {
			t.Fatalf("Save(%q) error = %v", id, err)
		}
	}
	for id, want := range urls {
		if got, err := s.Load(id); err != nil || got != want {
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
			if err := s.Save(id, "http://example.com"); err != nil {
				t.Errorf("Save(%q) error = %v", id, err)
			}
		}()
		go func() {
			defer wg.Done()
			_, _ = s.Load(id)
		}()
	}
	wg.Wait()
	for i := range n {
		id := fmt.Sprintf("id%d", i)
		if _, err := s.Load(id); err != nil {
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
			err := s.Save("same", fmt.Sprintf("http://example.com/%d", i))
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
