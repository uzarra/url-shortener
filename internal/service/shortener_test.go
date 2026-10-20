package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/uzarra/url-shortener/internal/model"
	"github.com/uzarra/url-shortener/internal/repository"
)

func TestNewShortener(t *testing.T) {
	type args struct {
		repo    Repository
		baseURL string
	}
	tests := []struct {
		name string
		args args
		want *Shortener
	}{
		{
			name: "success",
			args: args{
				repo:    newTestStorage(make(map[string]string)),
				baseURL: "http://localhost:8080/",
			},
			want: &Shortener{
				repo:    newTestStorage(make(map[string]string)),
				baseURL: "http://localhost:8080",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewShortener(tt.args.repo, tt.args.baseURL); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewShortener() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShortener_Expand(t *testing.T) {
	mem := make(map[string]string)
	mem["KEY"] = "VALUE"
	type fields struct {
		repo    Repository
		baseURL string
	}
	type args struct {
		id string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "success",
			fields: fields{
				repo:    newTestStorage(mem),
				baseURL: "http://localhost:8080/",
			},
			args: args{
				id: "KEY",
			},
			want:    "VALUE",
			wantErr: false,
		},
		{
			name: "no such key",
			fields: fields{
				repo:    newTestStorage(mem),
				baseURL: "http://localhost:8080/",
			},
			args: args{
				id: "ANOTHER_KEY",
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "invalid id",
			fields: fields{
				repo:    newTestStorage(mem),
				baseURL: "http://localhost:8080/",
			},
			args: args{
				id: "",
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Shortener{
				repo:    tt.fields.repo,
				baseURL: tt.fields.baseURL,
			}
			got, err := s.Expand(context.Background(), tt.args.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("Expand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Expand() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShortener_Shorten(t *testing.T) {
	type fields struct {
		repo    Repository
		baseURL string
	}
	type args struct {
		url string
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantPrefix string
		wantErr    bool
	}{
		{
			name: "success",
			fields: fields{
				repo:    newTestStorage(map[string]string{}),
				baseURL: "http://localhost:8080/",
			},
			args: args{
				url: "https://google.com",
			},
			wantPrefix: "http://localhost:8080/",
			wantErr:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Shortener{
				repo:    tt.fields.repo,
				baseURL: tt.fields.baseURL,
			}
			got, err := s.Shorten(context.Background(), tt.args.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("Shorten() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if strings.HasPrefix(got, tt.wantPrefix) != true {
				t.Errorf("Shorten() got = %v, wanted prefix %v", got, tt.wantPrefix)
			}
		})
	}
}

func TestShortener_ShortenBatch(t *testing.T) {
	storage := newTestStorage(map[string]string{})
	s := NewShortener(storage, "http://localhost:8080/")
	items := []model.BatchItem{
		{CorrelationID: "a1", OriginalURL: "http://yandex.ru"},
		{CorrelationID: "b2", OriginalURL: "http://ya.ru"},
	}
	got, err := s.ShortenBatch(context.Background(), items)
	if err != nil {
		t.Fatalf("ShortenBatch() error = %v", err)
	}
	if len(got) != len(items) {
		t.Fatalf("ShortenBatch() results = %d, want %d", len(got), len(items))
	}
	seen := make(map[string]struct{}, len(got))
	for i, res := range got {
		if res.CorrelationID != items[i].CorrelationID {
			t.Errorf("result[%d] correlation_id = %q, want %q", i, res.CorrelationID, items[i].CorrelationID)
		}
		id, ok := strings.CutPrefix(res.ShortURL, "http://localhost:8080/")
		if !ok || len(id) != idLength {
			t.Errorf("result[%d] short_url = %q, want base URL + id of length %d", i, res.ShortURL, idLength)
			continue
		}
		if _, dup := seen[id]; dup {
			t.Errorf("result[%d] id %q is duplicated", i, id)
		}
		seen[id] = struct{}{}
		if original, err := storage.Load(context.Background(), id); err != nil || original != items[i].OriginalURL {
			t.Errorf("Load(%q) = %q, %v, want %q, nil", id, original, err, items[i].OriginalURL)
		}
	}
}

func TestShortener_ShortenBatch_IncorrectInput(t *testing.T) {
	tests := []struct {
		name  string
		items []model.BatchItem
	}{
		{name: "nil batch", items: nil},
		{name: "empty batch", items: []model.BatchItem{}},
		{name: "empty url", items: []model.BatchItem{
			{CorrelationID: "a1", OriginalURL: "http://yandex.ru"},
			{CorrelationID: "b2", OriginalURL: " "},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := newTestStorage(map[string]string{})
			s := NewShortener(storage, "http://localhost:8080/")
			if _, err := s.ShortenBatch(context.Background(), tt.items); !errors.Is(err, ErrIncorrectInput) {
				t.Fatalf("ShortenBatch() error = %v, want %v", err, ErrIncorrectInput)
			}
			if len(storage.urls) != 0 {
				t.Errorf("stored urls = %d, want 0", len(storage.urls))
			}
		})
	}
}

func TestShortener_ShortenBatch_Retries(t *testing.T) {
	errDB := errors.New("db is down")
	tests := []struct {
		name      string
		batchErrs []error
		wantErr   error
		wantCalls int
	}{
		{name: "collision then success", batchErrs: []error{repository.ErrAlreadyExists}, wantCalls: 2},
		{name: "collisions exhaust retries",
			batchErrs: []error{repository.ErrAlreadyExists, repository.ErrAlreadyExists, repository.ErrAlreadyExists,
				repository.ErrAlreadyExists, repository.ErrAlreadyExists},
			wantErr: ErrIDGeneration, wantCalls: maxIDGenerateRetries},
		{name: "storage error is not retried", batchErrs: []error{errDB}, wantErr: errDB, wantCalls: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &failingBatchStorage{testStorage: newTestStorage(map[string]string{}), errs: tt.batchErrs}
			s := NewShortener(storage, "http://localhost:8080/")
			got, err := s.ShortenBatch(context.Background(), []model.BatchItem{
				{CorrelationID: "a1", OriginalURL: "http://yandex.ru"},
			})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ShortenBatch() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && len(got) != 1 {
				t.Errorf("ShortenBatch() results = %d, want 1", len(got))
			}
			if storage.calls != tt.wantCalls {
				t.Errorf("SaveBatch() calls = %d, want %d", storage.calls, tt.wantCalls)
			}
		})
	}
}

func Test_generateID(t *testing.T) {
	tests := []struct {
		name       string
		wantLength int
		wantErr    bool
	}{
		{
			name:       "success",
			wantLength: 8,
			wantErr:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := generateID()
			if (err != nil) != tt.wantErr {
				t.Errorf("generateID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) != tt.wantLength {
				t.Errorf("generateID() got = %v, wanted length = %v", got, tt.wantLength)
			}
		})
	}
}

type testStorage struct {
	mu   sync.RWMutex
	urls map[string]string
}

func newTestStorage(m map[string]string) *testStorage {
	return &testStorage{
		urls: m,
	}
}

func (t *testStorage) Save(ctx context.Context, id string, originalURL string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.urls[id]; ok {
		return repository.ErrAlreadyExists
	}
	t.urls[id] = originalURL
	return nil
}

func (t *testStorage) Load(ctx context.Context, id string) (originalURL string, err error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	originalURL, ok := t.urls[id]
	if !ok {
		return "", errors.New("not found")
	}
	return originalURL, nil
}

func (t *testStorage) SaveBatch(ctx context.Context, urls []model.URL) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	seen := make(map[string]struct{}, len(urls))
	for _, u := range urls {
		if _, ok := t.urls[u.ShortID]; ok {
			return repository.ErrAlreadyExists
		}
		if _, ok := seen[u.ShortID]; ok {
			return repository.ErrAlreadyExists
		}
		seen[u.ShortID] = struct{}{}
	}
	for _, u := range urls {
		t.urls[u.ShortID] = u.OriginalURL
	}
	return nil
}

type failingBatchStorage struct {
	*testStorage
	errs  []error
	calls int
}

func (f *failingBatchStorage) SaveBatch(ctx context.Context, urls []model.URL) error {
	f.calls++
	if f.calls <= len(f.errs) {
		return f.errs[f.calls-1]
	}
	return f.testStorage.SaveBatch(ctx, urls)
}
