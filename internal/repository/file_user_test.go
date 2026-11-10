package repository

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/uzarra/url-shortener/internal/model"
)

func fileGetByUser(t *testing.T, s *FileStorage, userID string) []model.URL {
	t.Helper()
	got, err := s.GetURLsByUserID(t.Context(), userID)
	if err != nil {
		t.Fatalf("GetURLsByUserID(%q) error = %v", userID, err)
	}
	return got
}

func readRawRecords(t *testing.T, path string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	var raw []map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal file: %v", err)
	}
	return raw
}

func TestFileStorage_GetURLsByUserID(t *testing.T) {
	s := newTestFileStorage(t, filepath.Join(t.TempDir(), "storage.json"))
	a1 := model.URL{ShortID: "a1", OriginalURL: "http://a1.ru", UserID: testUserA}
	b1 := model.URL{ShortID: "b1", OriginalURL: "http://b1.ru", UserID: testUserB}
	for _, u := range []model.URL{a1, b1} {
		if err := s.Save(t.Context(), u); err != nil {
			t.Fatalf("Save(%+v) error = %v", u, err)
		}
	}

	if got, want := fileGetByUser(t, s, testUserA), []model.URL{a1}; !reflect.DeepEqual(got, want) {
		t.Fatalf("GetURLsByUserID(A) = %+v, want %+v", got, want)
	}
}

func TestFileStorage_UserURLsSurviveRestore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "storage.json")
	s := newTestFileStorage(t, path)

	saved := []model.URL{
		{ShortID: "a1", OriginalURL: "http://a1.ru", UserID: testUserA},
		{ShortID: "anon", OriginalURL: "http://anon.ru"},
	}
	for _, u := range saved {
		if err := s.Save(t.Context(), u); err != nil {
			t.Fatalf("Save(%+v) error = %v", u, err)
		}
	}
	batch := []model.URL{
		{ShortID: "a2", OriginalURL: "http://a2.ru", UserID: testUserA},
		{ShortID: "b1", OriginalURL: "http://b1.ru", UserID: testUserB},
	}
	if err := s.SaveBatch(t.Context(), batch); err != nil {
		t.Fatalf("SaveBatch() error = %v", err)
	}

	restored := newTestFileStorage(t, path)

	wantA := []model.URL{saved[0], batch[0]}
	if got := fileGetByUser(t, restored, testUserA); !reflect.DeepEqual(got, wantA) {
		t.Fatalf("after restore GetURLsByUserID(A) = %+v, want %+v", got, wantA)
	}
	wantB := []model.URL{batch[1]}
	if got := fileGetByUser(t, restored, testUserB); !reflect.DeepEqual(got, wantB) {
		t.Fatalf("after restore GetURLsByUserID(B) = %+v, want %+v", got, wantB)
	}
	if got, err := restored.Load(t.Context(), "anon"); err != nil || got != "http://anon.ru" {
		t.Fatalf("after restore Load(anon) = %q, %v; want %q, nil", got, err, "http://anon.ru")
	}
}

func TestFileStorage_RestoreLegacyFileWithoutUserID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "storage.json")
	if err := os.WriteFile(path, []byte(validRecords), 0o644); err != nil {
		t.Fatalf("prepare file: %v", err)
	}

	s := newTestFileStorage(t, path)

	if got, err := s.Load(t.Context(), "4rSPg8ap"); err != nil || got != "http://yandex.ru" {
		t.Fatalf("Load() = %q, %v; want %q, nil", got, err, "http://yandex.ru")
	}
	if got := fileGetByUser(t, s, ""); len(got) != 0 {
		t.Fatalf("GetURLsByUserID(\"\") = %+v, legacy records must not belong to anyone", got)
	}

	u := model.URL{ShortID: "a1", OriginalURL: "http://a1.ru", UserID: testUserA}
	if err := s.Save(t.Context(), u); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	restored := newTestFileStorage(t, path)
	if got := fileGetByUser(t, restored, testUserA); !reflect.DeepEqual(got, []model.URL{u}) {
		t.Fatalf("GetURLsByUserID(A) = %+v, want %+v", got, []model.URL{u})
	}
}

func TestFileStorage_UserIDInFileFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "storage.json")
	s := newTestFileStorage(t, path)
	if err := s.Save(t.Context(), model.URL{ShortID: "anon", OriginalURL: "http://anon.ru"}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := s.SaveBatch(t.Context(), []model.URL{
		{ShortID: "a1", OriginalURL: "http://a1.ru", UserID: testUserA},
	}); err != nil {
		t.Fatalf("SaveBatch() error = %v", err)
	}

	raw := readRawRecords(t, path)
	if len(raw) != 2 {
		t.Fatalf("records = %d, want 2", len(raw))
	}
	if _, ok := raw[0]["user_id"]; ok {
		t.Errorf("record without user has user_id key: %v", raw[0])
	}
	if got := raw[1]["user_id"]; got != testUserA {
		t.Errorf("record user_id = %v, want %q", got, testUserA)
	}
}
