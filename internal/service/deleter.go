package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/uzarra/url-shortener/internal/model"
)

var (
	ErrDeleterClosed = errors.New("deleter closed")
)

const (
	flushTimeout = 5 * time.Second
)

type URLDeleter interface {
	DeleteURLs(ctx context.Context, reqs []model.DeleteRequest) error
}

type Deleter struct {
	repo          URLDeleter
	log           zerolog.Logger
	batchSize     int
	flushInterval time.Duration
	merged        chan model.DeleteRequest
	mu            sync.RWMutex
	closed        bool
	producers     sync.WaitGroup
	done          chan struct{}
	sem           chan struct{}
}

func NewDeleter(repo URLDeleter, log zerolog.Logger, batchSize int, flushInterval time.Duration, maxWorkers int) *Deleter {
	deleter := &Deleter{
		repo:          repo,
		log:           log,
		batchSize:     batchSize,
		flushInterval: flushInterval,
		merged:        make(chan model.DeleteRequest, batchSize),
		done:          make(chan struct{}),
		sem:           make(chan struct{}, maxWorkers),
	}
	go deleter.run()
	return deleter
}

func (d *Deleter) run() {
	defer close(d.done)
	ticker := time.NewTicker(d.flushInterval)
	defer ticker.Stop()
	buf := make([]model.DeleteRequest, 0, d.batchSize)

	for {
		select {
		case req, ok := <-d.merged:
			if !ok {
				d.flush(buf)
				return
			}
			buf = append(buf, req)
			if len(buf) >= d.batchSize {
				d.flush(buf)
				buf = make([]model.DeleteRequest, 0, d.batchSize)
			}
		case <-ticker.C:
			if len(buf) > 0 {
				d.flush(buf)
				buf = make([]model.DeleteRequest, 0, d.batchSize)
			}
		}
	}
}

func (d *Deleter) Delete(ctx context.Context, userID string, ids []string) error {
	if userID == "" || len(ids) == 0 {
		return nil
	}

	select {
	case d.sem <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}

	d.mu.RLock()
	if d.closed {
		d.mu.RUnlock()
		<-d.sem
		return ErrDeleterClosed
	}
	d.producers.Add(1)
	d.mu.RUnlock()

	go func() {
		defer d.producers.Done()
		defer func() { <-d.sem }()
		for _, id := range ids {
			d.merged <- model.DeleteRequest{UserID: userID, ShortID: id}
		}
	}()
	return nil
}

func (d *Deleter) flush(buf []model.DeleteRequest) {
	if len(buf) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), flushTimeout)
	defer cancel()
	if err := d.repo.DeleteURLs(ctx, buf); err != nil {
		d.log.Error().Err(err).Int("count", len(buf)).Msg("failed to delete urls batch")
	}
}

func (d *Deleter) Close() {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		<-d.done
		return
	}
	d.closed = true
	d.mu.Unlock()
	d.producers.Wait()
	close(d.merged)
	<-d.done
}
