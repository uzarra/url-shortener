package service

import (
	"context"
	"crypto/rand"
	"errors"
	"strings"

	"github.com/uzarra/url-shortener/internal/model"
	"github.com/uzarra/url-shortener/internal/repository"
)

const (
	idLength             = 8
	maxIDGenerateRetries = 5
	alphabet             = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

var ErrIncorrectInput = errors.New("input is incorrect")
var ErrIDGeneration = errors.New("id generation failed")
var ErrConflict = errors.New("url already shortened")

type Repository interface {
	Save(ctx context.Context, id string, url string) error
	Load(ctx context.Context, id string) (string, error)
	SaveBatch(ctx context.Context, urls []model.URL) error
	GetShortURLByOriginalURL(ctx context.Context, originalURL string) (string, error)
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
		if errors.Is(err, repository.ErrConflict) {
			existing, err := s.GetShortURLByOriginalURL(ctx, url)
			if err != nil {
				return "", err
			}
			return existing, ErrConflict
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

func (s *Shortener) ShortenBatch(ctx context.Context, batchItems []model.BatchItem) ([]model.BatchResult, bool, error) {
	if !isValid(batchItems) {
		return nil, false, ErrIncorrectInput
	}
	for range maxIDGenerateRetries {
		ids := make(map[string]string, len(batchItems))
		var conflict bool
		var urls []model.URL
		for _, item := range batchItems {
			if _, ok := ids[item.OriginalURL]; ok {
				continue
			}
			existing, err := s.repo.GetShortURLByOriginalURL(ctx, item.OriginalURL)
			if err == nil {
				ids[item.OriginalURL] = existing
				conflict = true
				continue
			}
			if !errors.Is(err, repository.ErrNotFound) {
				return nil, false, err
			}
			id, err := generateID()
			if err != nil {
				return nil, false, err
			}
			ids[item.OriginalURL] = id
			urls = append(urls, model.URL{ShortID: id, OriginalURL: item.OriginalURL})
		}
		err := s.repo.SaveBatch(ctx, urls)
		if errors.Is(err, repository.ErrAlreadyExists) || errors.Is(err, repository.ErrConflict) {
			continue
		}
		if err != nil {
			return nil, false, err
		}
		results := make([]model.BatchResult, len(batchItems))
		for i, item := range batchItems {
			results[i] = model.BatchResult{
				CorrelationID: item.CorrelationID,
				ShortURL:      s.baseURL + "/" + ids[item.OriginalURL],
			}
		}
		return results, conflict, nil
	}
	return nil, false, ErrIDGeneration
}

func (s *Shortener) GetShortURLByOriginalURL(ctx context.Context, originalURL string) (string, error) {
	if originalURL == "" {
		return "", ErrIncorrectInput
	}
	shortURL, err := s.repo.GetShortURLByOriginalURL(ctx, originalURL)
	if err != nil {
		return "", err
	}
	return s.baseURL + "/" + shortURL, nil
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

func isValid(request []model.BatchItem) bool {
	if len(request) == 0 {
		return false
	}
	for _, item := range request {
		if strings.TrimSpace(item.OriginalURL) == "" {
			return false
		}
	}
	return true
}
