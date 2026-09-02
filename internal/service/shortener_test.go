package service

import (
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestNewShortener(t *testing.T) {
	type args struct {
		repo    Repository
		baseUrl string
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
				baseUrl: "http://localhost:8080/",
			},
			want: &Shortener{
				repo:    newTestStorage(make(map[string]string)),
				baseUrl: "http://localhost:8080",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewShortener(tt.args.repo, tt.args.baseUrl); !reflect.DeepEqual(got, tt.want) {
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
		baseUrl string
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
				baseUrl: "http://localhost:8080/",
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
				baseUrl: "http://localhost:8080/",
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
				baseUrl: "http://localhost:8080/",
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
				baseUrl: tt.fields.baseUrl,
			}
			got, err := s.Expand(tt.args.id)
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
		baseUrl string
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
				baseUrl: "http://localhost:8080/",
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
				baseUrl: tt.fields.baseUrl,
			}
			got, err := s.Shorten(tt.args.url)
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

func Test_generateId(t *testing.T) {
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
			got, err := generateId()
			if (err != nil) != tt.wantErr {
				t.Errorf("generateId() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) != tt.wantLength {
				t.Errorf("generateId() got = %v, wanted length = %v", got, tt.wantLength)
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

func (t *testStorage) Save(id string, originalUrl string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.urls[id] = originalUrl
	return nil
}

func (t *testStorage) Load(id string) (originalUrl string, err error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	originalUrl, ok := t.urls[id]
	if !ok {
		return "", errors.New("not found")
	}
	return originalUrl, nil
}
