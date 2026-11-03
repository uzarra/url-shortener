package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/uzarra/url-shortener/internal/model"
)

const (
	testUserA = "11111111-1111-4111-8111-111111111111"
	testUserB = "22222222-2222-4222-8222-222222222222"
	testBase  = "http://localhost:8080"
)

func TestShortener_Shorten_SavesUserID(t *testing.T) {
	storage := newTestStorage(map[string]string{})
	s := NewShortener(storage, testBase+"/")

	shortURL, err := s.Shorten(context.Background(), "http://yandex.ru", testUserA)
	if err != nil {
		t.Fatalf("Shorten() error = %v", err)
	}

	got, err := s.GetUserURLs(context.Background(), testUserA)
	if err != nil {
		t.Fatalf("GetUserURLs() error = %v", err)
	}
	want := []model.UserURL{{ShortURL: shortURL, OriginalURL: "http://yandex.ru"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetUserURLs() = %+v, want %+v", got, want)
	}
}

func TestShortener_Shorten_WithoutUser(t *testing.T) {
	storage := newTestStorage(map[string]string{})
	s := NewShortener(storage, testBase+"/")

	if _, err := s.Shorten(context.Background(), "http://yandex.ru", ""); err != nil {
		t.Fatalf("Shorten() error = %v", err)
	}
	if len(storage.urls) != 1 {
		t.Fatalf("stored urls = %d, want 1", len(storage.urls))
	}
	if len(storage.byUser) != 0 {
		t.Fatalf("byUser = %v, url without user must not be indexed", storage.byUser)
	}
}

func TestShortener_ShortenBatch_SavesUserID(t *testing.T) {
	s := NewShortener(newTestStorage(map[string]string{}), testBase+"/")
	items := []model.BatchItem{
		{CorrelationID: "1", OriginalURL: "http://a.ru"},
		{CorrelationID: "2", OriginalURL: "http://b.ru"},
	}

	results, _, err := s.ShortenBatch(context.Background(), items, testUserA)
	if err != nil {
		t.Fatalf("ShortenBatch() error = %v", err)
	}

	got, err := s.GetUserURLs(context.Background(), testUserA)
	if err != nil {
		t.Fatalf("GetUserURLs() error = %v", err)
	}
	want := []model.UserURL{
		{ShortURL: results[0].ShortURL, OriginalURL: "http://a.ru"},
		{ShortURL: results[1].ShortURL, OriginalURL: "http://b.ru"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetUserURLs() = %+v, want %+v", got, want)
	}
}

func TestShortener_GetUserURLs_BuildsFullShortURL(t *testing.T) {
	storage := newTestStorage(map[string]string{})
	if err := storage.Save(context.Background(), model.URL{ShortID: "abc", OriginalURL: "http://yandex.ru", UserID: testUserA}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	// Базовый адрес со слешем на конце: в результате не должно быть двойного слеша.
	s := NewShortener(storage, testBase+"/")

	got, err := s.GetUserURLs(context.Background(), testUserA)
	if err != nil {
		t.Fatalf("GetUserURLs() error = %v", err)
	}
	want := []model.UserURL{{ShortURL: testBase + "/abc", OriginalURL: "http://yandex.ru"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetUserURLs() = %+v, want %+v", got, want)
	}
	if strings.Contains(strings.TrimPrefix(got[0].ShortURL, "http://"), "//") {
		t.Errorf("ShortURL %q contains double slash", got[0].ShortURL)
	}
}

func TestShortener_GetUserURLs_OnlyOwnURLs(t *testing.T) {
	s := NewShortener(newTestStorage(map[string]string{}), testBase+"/")
	if _, err := s.Shorten(context.Background(), "http://a.ru", testUserA); err != nil {
		t.Fatalf("Shorten(A) error = %v", err)
	}
	if _, err := s.Shorten(context.Background(), "http://b.ru", testUserB); err != nil {
		t.Fatalf("Shorten(B) error = %v", err)
	}

	got, err := s.GetUserURLs(context.Background(), testUserB)
	if err != nil {
		t.Fatalf("GetUserURLs() error = %v", err)
	}
	if len(got) != 1 || got[0].OriginalURL != "http://b.ru" {
		t.Fatalf("GetUserURLs(B) = %+v, want only http://b.ru", got)
	}
}

func TestShortener_GetUserURLs_NoURLs(t *testing.T) {
	s := NewShortener(newTestStorage(map[string]string{}), testBase+"/")

	got, err := s.GetUserURLs(context.Background(), testUserA)
	if err != nil {
		t.Fatalf("GetUserURLs() error = %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("GetUserURLs() = %#v, want empty non-nil slice", got)
	}
}

func TestShortener_GetUserURLs_EmptyUserID(t *testing.T) {
	s := NewShortener(newTestStorage(map[string]string{}), testBase+"/")

	if _, err := s.GetUserURLs(context.Background(), ""); !errors.Is(err, ErrIncorrectInput) {
		t.Fatalf("GetUserURLs(\"\") error = %v, want %v", err, ErrIncorrectInput)
	}
}

func TestShortener_GetUserURLs_RepoError(t *testing.T) {
	errDB := errors.New("db is down")
	storage := &failingUserURLsStorage{testStorage: newTestStorage(map[string]string{}), err: errDB}
	s := NewShortener(storage, testBase+"/")

	got, err := s.GetUserURLs(context.Background(), testUserA)
	if !errors.Is(err, errDB) {
		t.Fatalf("GetUserURLs() error = %v, want %v", err, errDB)
	}
	if got != nil {
		t.Errorf("GetUserURLs() = %+v, want nil on error", got)
	}
}
