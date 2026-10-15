package service

import (
	"context"
	"crypto/rand"
	"errors"
	"strings"

	"github.com/uzarra/url-shortener/internal/repository"
)

const (
	idLength             = 8
	maxIDGenerateRetries = 5
	alphabet             = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

var ErrIncorrectInput = errors.New("input is incorrect")
var ErrIDGeneration = errors.New("id generation failed")

type Repository interface {
	Save(ctx context.Context, id string, url string) error
	Load(ctx context.Context, id string) (string, error)
}

type Shortener struct {
	repo    Repository
	baseURL string
}

func NewShortener(repo Repository, baseURL string) *Shortener {
	return &Shortener{
		repo:    repo,
		baseURL: strings.TrimRight(baseURL, "/"),
	}
}

func (s *Shortener) Shorten(ctx context.Context, url string) (string, error) {
	if url == "" {
		return "", ErrIncorrectInput
	}
	for range maxIDGenerateRetries {
		id, err := generateID()
		if err != nil {
			return "", err
		}
		err = s.repo.Save(ctx, id, url)
		if errors.Is(err, repository.ErrAlreadyExists) {
			continue
		}
		if err != nil {
			return "", err
		}
		return s.baseURL + "/" + id, nil
	}
	return "", ErrIDGeneration
}

func (s *Shortener) Expand(ctx context.Context, id string) (string, error) {
	if id == "" {
		return "", ErrIncorrectInput
	}
	originalURL, err := s.repo.Load(ctx, id)
	if err != nil {
		return "", err
	}
	return originalURL, nil
}

func generateID() (string, error) {
	buf := make([]byte, idLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for i, b := range buf {
		buf[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(buf), nil
}
