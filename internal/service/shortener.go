package service

import (
	"crypto/rand"
	"errors"
	"strings"
)

const (
	IDLength             = 8
	MaxIDGenerateRetries = 5
	Alphabet             = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

var ErrIncorrectInput = errors.New("input is incorrect")
var ErrIDGeneration = errors.New("id generation failed")

type Repository interface {
	Save(id string, url string) error
	Load(id string) (string, error)
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

func (s *Shortener) Shorten(url string) (string, error) {
	if url == "" {
		return "", ErrIncorrectInput
	}
	for range MaxIDGenerateRetries {
		id, err := generateID()
		if err != nil {
			return "", err
		}
		if _, err := s.repo.Load(id); err == nil {
			continue
		}
		if err := s.repo.Save(id, url); err != nil {
			return "", err
		}
		return s.baseURL + "/" + id, nil
	}
	return "", ErrIDGeneration
}

func (s *Shortener) Expand(id string) (string, error) {
	if id == "" {
		return "", ErrIncorrectInput
	}
	originalURL, err := s.repo.Load(id)
	if err != nil {
		return "", err
	}
	return originalURL, nil
}

func generateID() (string, error) {
	buf := make([]byte, IDLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for i, b := range buf {
		buf[i] = Alphabet[int(b)%len(Alphabet)]
	}
	return string(buf), nil
}
