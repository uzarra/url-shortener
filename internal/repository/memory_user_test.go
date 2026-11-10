package repository

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/uzarra/url-shortener/internal/model"
)

const (
	testUserA = "11111111-1111-4111-8111-111111111111"
	testUserB = "22222222-2222-4222-8222-222222222222"
)

func mustSave(t *testing.T, s *Storage, u model.URL) {
	t.Helper()
	if err := s.Save(t.Context(), u); err != nil {
		t.Fatalf("Save(%+v) error = %v", u, err)
	}
}

func mustGetByUser(t *testing.T, s *Storage, userID string) []model.URL {
	t.Helper()
	got, err := s.GetURLsByUserID(t.Context(), userID)
	if err != nil {
		t.Fatalf("GetURLsByUserID(%q) error = %v", userID, err)
	}
	return got
}

func TestStorage_GetURLsByUserID_OwnURLsInOrder(t *testing.T) {
	s := NewStorage()
	mustSave(t, s, model.URL{ShortID: "a1", OriginalURL: "http://a1.ru", UserID: testUserA})
	mustSave(t, s, model.URL{ShortID: "b1", OriginalURL: "http://b1.ru", UserID: testUserB})
	mustSave(t, s, model.URL{ShortID: "a2", OriginalURL: "http://a2.ru", UserID: testUserA})

	want := []model.URL{
		{ShortID: "a1", OriginalURL: "http://a1.ru", UserID: testUserA},
		{ShortID: "a2", OriginalURL: "http://a2.ru", UserID: testUserA},
	}
	if got := mustGetByUser(t, s, testUserA); !reflect.DeepEqual(got, want) {
		t.Fatalf("GetURLsByUserID(A) = %+v, want %+v", got, want)
	}

	wantB := []model.URL{{ShortID: "b1", OriginalURL: "http://b1.ru", UserID: testUserB}}
	if got := mustGetByUser(t, s, testUserB); !reflect.DeepEqual(got, wantB) {
		t.Fatalf("GetURLsByUserID(B) = %+v, want %+v", got, wantB)
	}
}

func TestStorage_GetURLsByUserID_UnknownUser(t *testing.T) {
	s := NewStorage()
	mustSave(t, s, model.URL{ShortID: "a1", OriginalURL: "http://a1.ru", UserID: testUserA})

	got := mustGetByUser(t, s, testUserB)
	if got == nil {
		t.Fatal("GetURLsByUserID() = nil, want empty non-nil slice")
	}
	if len(got) != 0 {
		t.Fatalf("GetURLsByUserID() = %+v, want empty", got)
	}
}

func TestStorage_GetURLsByUserID_URLWithoutUser(t *testing.T) {
	s := NewStorage()
	mustSave(t, s, model.URL{ShortID: "anon", OriginalURL: "http://anon.ru"})

	if got := mustGetByUser(t, s, ""); len(got) != 0 {
		t.Fatalf("GetURLsByUserID(\"\") = %+v, want empty", got)
	}
	if got := mustGetByUser(t, s, testUserA); len(got) != 0 {
		t.Fatalf("GetURLsByUserID(A) = %+v, want empty", got)
	}
	if _, err := s.Load(t.Context(), "anon"); err != nil {
		t.Fatalf("Load(anon) error = %v, url without user must still be stored", err)
	}
}

func TestStorage_GetURLsByUserID_SaveBatch(t *testing.T) {
	s := NewStorage()
	batch := []model.URL{
		{ShortID: "a1", OriginalURL: "http://a1.ru", UserID: testUserA},
		{ShortID: "b1", OriginalURL: "http://b1.ru", UserID: testUserB},
		{ShortID: "a2", OriginalURL: "http://a2.ru", UserID: testUserA},
		{ShortID: "x1", OriginalURL: "http://x1.ru"},
	}
	if err := s.SaveBatch(t.Context(), batch); err != nil {
		t.Fatalf("SaveBatch() error = %v", err)
	}

	want := []model.URL{batch[0], batch[2]}
	if got := mustGetByUser(t, s, testUserA); !reflect.DeepEqual(got, want) {
		t.Fatalf("GetURLsByUserID(A) = %+v, want %+v", got, want)
	}
}

func TestStorage_GetURLsByUserID_FailedSaveNotIndexed(t *testing.T) {
	s := NewStorage()
	mustSave(t, s, model.URL{ShortID: "abc", OriginalURL: "http://yandex.ru", UserID: testUserA})

	if err := s.Save(t.Context(), model.URL{ShortID: "abc", OriginalURL: "http://ya.ru", UserID: testUserB}); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("Save() duplicate id error = %v, want %v", err, ErrAlreadyExists)
	}
	if err := s.Save(t.Context(), model.URL{ShortID: "def", OriginalURL: "http://yandex.ru", UserID: testUserB}); !errors.Is(err, ErrConflict) {
		t.Fatalf("Save() conflict error = %v, want %v", err, ErrConflict)
	}

	if got := mustGetByUser(t, s, testUserB); len(got) != 0 {
		t.Fatalf("GetURLsByUserID(B) = %+v, failed saves must not be indexed", got)
	}
}

func TestStorage_GetURLsByUserID_FailedBatchNotIndexed(t *testing.T) {
	s := NewStorage()
	mustSave(t, s, model.URL{ShortID: "abc", OriginalURL: "http://yandex.ru"})

	err := s.SaveBatch(t.Context(), []model.URL{
		{ShortID: "new1", OriginalURL: "http://new1.ru", UserID: testUserA},
		{ShortID: "abc", OriginalURL: "http://new2.ru", UserID: testUserA},
	})
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("SaveBatch() error = %v, want %v", err, ErrAlreadyExists)
	}

	if got := mustGetByUser(t, s, testUserA); len(got) != 0 {
		t.Fatalf("GetURLsByUserID(A) = %+v, failed batch must not be indexed", got)
	}
}

func TestStorage_GetURLsByUserID_ResultIsCopy(t *testing.T) {
	s := NewStorage()
	mustSave(t, s, model.URL{ShortID: "a1", OriginalURL: "http://a1.ru", UserID: testUserA})

	got := mustGetByUser(t, s, testUserA)
	got[0].ShortID = "hacked"
	_ = append(got[:0], model.URL{ShortID: "injected"})

	again := mustGetByUser(t, s, testUserA)
	if len(again) != 1 || again[0].ShortID != "a1" {
		t.Fatalf("GetURLsByUserID() after mutating result = %+v, storage must not change", again)
	}
}

func TestStorage_GetURLsByUserID_Concurrent(t *testing.T) {
	s := NewStorage()
	const n = 100

	var wg sync.WaitGroup
	for i := range n {
		wg.Add(2)
		go func() {
			defer wg.Done()
			id := fmt.Sprintf("id%d", i)
			if err := s.Save(t.Context(), model.URL{ShortID: id, OriginalURL: "http://example" + id + ".com", UserID: testUserA}); err != nil {
				t.Errorf("Save(%q) error = %v", id, err)
			}
		}()
		go func() {
			defer wg.Done()
			if _, err := s.GetURLsByUserID(t.Context(), testUserA); err != nil {
				t.Errorf("GetURLsByUserID() error = %v", err)
			}
		}()
	}
	wg.Wait()

	if got := mustGetByUser(t, s, testUserA); len(got) != n {
		t.Fatalf("GetURLsByUserID() returned %d urls, want %d", len(got), n)
	}
}
