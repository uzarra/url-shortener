package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func newTestFileStorage(t *testing.T, path string) *FileStorage {
	t.Helper()
	s, err := NewFileStorage(path)
	if err != nil {
		t.Fatalf("NewFileStorage() error = %v", err)
	}
	return s
}

func readRecords(t *testing.T, path string) []record {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	var records []record
	if err := json.Unmarshal(data, &records); err != nil {
		t.Fatalf("unmarshal file: %v", err)
	}
	return records
}

func TestNewFileStorage(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
		create  bool
		wantErr bool
	}{
		{name: "file does not exist"},
		{name: "empty file", create: true},
		{name: "empty array", content: []byte("[]"), create: true},
		{name: "invalid json", content: []byte("not json"), create: true, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "storage.json")
			if tt.create {
				if err := os.WriteFile(path, tt.content, 0o644); err != nil {
					t.Fatalf("prepare file: %v", err)
				}
			}
			s, err := NewFileStorage(path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewFileStorage() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if _, err := s.Load("missing"); !errors.Is(err, ErrNotFound) {
				t.Errorf("Load() error = %v, want %v", err, ErrNotFound)
			}
		})
	}
}

func TestFileStorage_SaveLoad(t *testing.T) {
	s := newTestFileStorage(t, filepath.Join(t.TempDir(), "storage.json"))
	if err := s.Save("abc", "http://yandex.ru"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := s.Load("abc")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got != "http://yandex.ru" {
		t.Errorf("Load() = %q, want %q", got, "http://yandex.ru")
	}
}

func TestFileStorage_SaveDuplicate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "storage.json")
	s := newTestFileStorage(t, path)
	if err := s.Save("abc", "http://yandex.ru"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := s.Save("abc", "http://ya.ru"); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("Save() duplicate error = %v, want %v", err, ErrAlreadyExists)
	}
	if got, _ := s.Load("abc"); got != "http://yandex.ru" {
		t.Errorf("Load() = %q, want original %q", got, "http://yandex.ru")
	}
	if records := readRecords(t, path); len(records) != 1 {
		t.Errorf("records in file = %d, want 1", len(records))
	}
}

func TestFileStorage_Restore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "storage.json")
	urls := map[string]string{
		"4rSPg8ap": "http://yandex.ru",
		"edVPg3ks": "http://ya.ru",
		"dG56Hqxm": "http://practicum.yandex.ru",
	}
	s := newTestFileStorage(t, path)
	for id, u := range urls {
		if err := s.Save(id, u); err != nil {
			t.Fatalf("Save(%q) error = %v", id, err)
		}
	}

	restored := newTestFileStorage(t, path)
	for id, want := range urls {
		got, err := restored.Load(id)
		if err != nil {
			t.Errorf("Load(%q) error = %v", id, err)
			continue
		}
		if got != want {
			t.Errorf("Load(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestFileStorage_FileFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "storage.json")
	s := newTestFileStorage(t, path)
	if err := s.Save("4rSPg8ap", "http://yandex.ru"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := s.Save("edVPg3ks", "http://ya.ru"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	want := []record{
		{UUID: "1", ShortURL: "4rSPg8ap", OriginalURL: "http://yandex.ru"},
		{UUID: "2", ShortURL: "edVPg3ks", OriginalURL: "http://ya.ru"},
	}
	got := readRecords(t, path)
	if len(got) != len(want) {
		t.Fatalf("records = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("record[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o644 {
		t.Errorf("file perm = %o, want %o", perm, 0o644)
	}
	if _, err := os.Stat(path + ".tmp"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("tmp file should not remain, stat error = %v", err)
	}
}

func TestFileStorage_UUIDContinuesAfterRestore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "storage.json")
	s := newTestFileStorage(t, path)
	_ = s.Save("a", "http://a.ru")
	_ = s.Save("b", "http://b.ru")

	restored := newTestFileStorage(t, path)
	if err := restored.Save("c", "http://c.ru"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	records := readRecords(t, path)
	if len(records) != 3 {
		t.Fatalf("records = %d, want 3", len(records))
	}
	if records[2].UUID != "3" {
		t.Errorf("uuid = %q, want %q", records[2].UUID, "3")
	}
}

func TestNewFileStorage_CreatesDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "storage.json")
	s := newTestFileStorage(t, path)
	if err := s.Save("abc", "http://yandex.ru"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if records := readRecords(t, path); len(records) != 1 {
		t.Errorf("records = %d, want 1", len(records))
	}
}

func TestNewFileStorage_DirCreateError(t *testing.T) {
	notDir := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(notDir, nil, 0o644); err != nil {
		t.Fatalf("prepare file: %v", err)
	}
	if _, err := NewFileStorage(filepath.Join(notDir, "storage.json")); err == nil {
		t.Fatal("NewFileStorage() error = nil, want error")
	}
}

func TestFileStorage_SaveWriteError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "storage.json")
	s := newTestFileStorage(t, path)
	// a directory in place of the tmp file makes os.WriteFile fail
	if err := os.Mkdir(path+".tmp", 0o755); err != nil {
		t.Fatalf("prepare tmp dir: %v", err)
	}
	if err := s.Save("abc", "http://yandex.ru"); err == nil {
		t.Fatal("Save() error = nil, want error")
	}
	if _, err := s.Load("abc"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Load() after failed Save error = %v, want %v", err, ErrNotFound)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("storage file should not exist after failed Save, stat error = %v", err)
	}
}

func TestFileStorage_ConcurrentSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "storage.json")
	s := newTestFileStorage(t, path)
	const n = 50
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := fmt.Sprintf("id%d", i)
			if err := s.Save(id, "http://example.com"); err != nil {
				t.Errorf("Save(%q) error = %v", id, err)
			}
		}()
	}
	wg.Wait()
	if records := readRecords(t, path); len(records) != n {
		t.Errorf("records = %d, want %d", len(records), n)
	}
}
