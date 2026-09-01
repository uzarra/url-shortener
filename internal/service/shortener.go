package service

import (
	"crypto/rand"
	"errors"
	"strings"
)

const (
	IdLength             = 8
	MaxIdGenerateRetries = 5
	Alphabet             = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

var ErrIncorrectInput = errors.New("input is incorrect")
var ErrIdGeneration = errors.New("id generation failed")

type Repository interface {
	Save(id string, url string) error
	Load(id string) (string, error)
}

type Shortener struct {
	repo    Repository
	baseUrl string
}

func NewShortener(repo Repository, baseUrl string) *Shortener {
	return &Shortener{
		repo:    repo,
		baseUrl: strings.TrimRight(baseUrl, "/"),
	}
}

func (s *Shortener) Shorten(url string) (string, error) {
	if url == "" {
		return "", ErrIncorrectInput
	}
	for range MaxIdGenerateRetries {
		id, err := generateId()
		if err != nil {
			return "", err
		}
		if _, err := s.repo.Load(id); err == nil {
			continue
		}
		if err := s.repo.Save(id, url); err != nil {
			return "", err
		}
		return s.baseUrl + "/" + id, nil
	}
	return "", ErrIdGeneration
}

func (s *Shortener) Expand(id string) (string, error) {
	if id == "" {
		return "", ErrIncorrectInput
	}
	originalUrl, err := s.repo.Load(id)
	if err != nil {
		return "", err
	}
	return originalUrl, nil
}

func generateId() (string, error) {
	buf := make([]byte, IdLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for i, b := range buf {
		buf[i] = Alphabet[int(b)%len(Alphabet)]
	}
	return string(buf), nil
}
