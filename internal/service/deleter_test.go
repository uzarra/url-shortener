package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uzarra/url-shortener/internal/model"
)

type fakeURLDeleter struct {
	mu      sync.Mutex
	batches [][]model.DeleteRequest
	err     error
}

func (f *fakeURLDeleter) DeleteURLs(_ context.Context, reqs []model.DeleteRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.batches = append(f.batches, reqs)
	return f.err
}

func flatten(batches [][]model.DeleteRequest) []model.DeleteRequest {
	var all []model.DeleteRequest
	for _, b := range batches {
		all = append(all, b...)
	}
	return all
}

func (f *fakeURLDeleter) snapshot() [][]model.DeleteRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([][]model.DeleteRequest(nil), f.batches...)
}

func TestDeleter_DeliversAllOnClose(t *testing.T) {
	repo := &fakeURLDeleter{}
	d := NewDeleter(repo, zerolog.Nop(), 100, time.Hour, 10)
	require.NoError(t, d.Delete(t.Context(), "u1", []string{"a", "b"}))
	require.NoError(t, d.Delete(t.Context(), "u2", []string{"c"}))
	d.Close()
	assert.ElementsMatch(t, []model.DeleteRequest{
		{UserID: "u1", ShortID: "a"},
		{UserID: "u1", ShortID: "b"},
		{UserID: "u2", ShortID: "c"},
	}, flatten(repo.snapshot()))
}

func TestDeleter_Batching(t *testing.T) {
	repo := &fakeURLDeleter{}
	d := NewDeleter(repo, zerolog.Nop(), 3, time.Hour, 10)
	require.NoError(t, d.Delete(t.Context(), "u1", []string{"1", "2", "3", "4", "5", "6", "7"}))
	d.Close()
	var sizes []int
	for _, b := range repo.snapshot() {
		sizes = append(sizes, len(b))
	}
	assert.Equal(t, []int{3, 3, 1}, sizes)
}

func TestDeleter_FlushByTicker(t *testing.T) {
	repo := &fakeURLDeleter{}
	d := NewDeleter(repo, zerolog.Nop(), 100, 10*time.Millisecond, 10)
	defer d.Close()
	require.NoError(t, d.Delete(t.Context(), "u1", []string{"a"}))
	require.Eventually(t, func() bool { return len(flatten(repo.snapshot())) == 1 },
		time.Second, 5*time.Millisecond)
}

func TestDeleter_AfterClose(t *testing.T) {
	d := NewDeleter(&fakeURLDeleter{}, zerolog.Nop(), 10, time.Hour, 10)
	d.Close()
	assert.ErrorIs(t, d.Delete(t.Context(), "u1", []string{"a"}), ErrDeleterClosed)
	assert.NotPanics(t, d.Close)
}

func TestDeleter_RepoErrorDoesNotStopWorker(t *testing.T) {
	repo := &fakeURLDeleter{err: errors.New("db down")}
	d := NewDeleter(repo, zerolog.Nop(), 1, time.Hour, 10)
	require.NoError(t, d.Delete(t.Context(), "u1", []string{"a", "b"}))
	d.Close()
	assert.Len(t, repo.snapshot(), 2)
}

func TestDeleter_EmptyInput(t *testing.T) {
	repo := &fakeURLDeleter{}
	d := NewDeleter(repo, zerolog.Nop(), 10, time.Hour, 10)
	require.NoError(t, d.Delete(t.Context(), "", []string{"a"}))
	require.NoError(t, d.Delete(t.Context(), "u1", nil))
	d.Close()
	assert.Empty(t, repo.snapshot())
}

func TestDeleter_ConcurrentDeleteAndClose(t *testing.T) {
	d := NewDeleter(&fakeURLDeleter{}, zerolog.Nop(), 5, time.Millisecond, 10)
	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = d.Delete(t.Context(), "u1", []string{"a", "b"})
		}()
	}
	d.Close()
	wg.Wait()
}

type blockingURLDeleter struct {
	fakeURLDeleter
	gate chan struct{}
}

func (b *blockingURLDeleter) DeleteURLs(ctx context.Context, reqs []model.DeleteRequest) error {
	<-b.gate
	return b.fakeURLDeleter.DeleteURLs(ctx, reqs)
}

func TestDeleter_LimitsConcurrentWriters(t *testing.T) {
	const maxWorkers = 2
	repo := &blockingURLDeleter{gate: make(chan struct{})}
	d := NewDeleter(repo, zerolog.Nop(), 1, time.Hour, maxWorkers)

	require.NoError(t, d.Delete(t.Context(), "u1", []string{"a1", "a2", "a3"}))
	require.NoError(t, d.Delete(t.Context(), "u2", []string{"b1", "b2", "b3"}))

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	err := d.Delete(ctx, "u3", []string{"c1"})
	assert.ErrorIs(t, err, context.DeadlineExceeded, "writer over the limit must not start")

	close(repo.gate)
	d.Close()
	assert.ElementsMatch(t, []model.DeleteRequest{
		{UserID: "u1", ShortID: "a1"}, {UserID: "u1", ShortID: "a2"}, {UserID: "u1", ShortID: "a3"},
		{UserID: "u2", ShortID: "b1"}, {UserID: "u2", ShortID: "b2"}, {UserID: "u2", ShortID: "b3"},
	}, flatten(repo.snapshot()), "rejected request must not be delivered, accepted ones must")
}

func TestDeleter_SlotIsReleasedAfterWriterFinishes(t *testing.T) {
	repo := &fakeURLDeleter{}
	d := NewDeleter(repo, zerolog.Nop(), 100, time.Hour, 1)

	for i := range 5 {
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		err := d.Delete(ctx, "u1", []string{"id"})
		cancel()
		require.NoError(t, err, "call %d: slot of previous writer was not released", i)
	}
	d.Close()
	assert.Len(t, flatten(repo.snapshot()), 5)
}

func TestDeleter_SlotIsReleasedWhenClosed(t *testing.T) {
	d := NewDeleter(&fakeURLDeleter{}, zerolog.Nop(), 10, time.Hour, 1)
	d.Close()

	for range 2 {
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		err := d.Delete(ctx, "u1", []string{"a"})
		cancel()
		assert.ErrorIs(t, err, ErrDeleterClosed)
	}
}
