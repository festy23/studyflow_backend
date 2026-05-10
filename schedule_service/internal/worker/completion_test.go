package worker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeRepo struct {
	calls atomic.Int32
	err   error
	// signal lets the test know a call happened.
	signal chan struct{}
}

func (f *fakeRepo) UpdateCompletedLessons(ctx context.Context) (int, error) {
	f.calls.Add(1)
	if f.signal != nil {
		select {
		case f.signal <- struct{}{}:
		default:
		}
	}
	return 0, f.err
}

func TestLessonCompletionWorker_TickCallsRepo(t *testing.T) {
	repo := &fakeRepo{signal: make(chan struct{}, 4)}
	w := NewLessonCompletionWorker(repo, 10*time.Millisecond, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	w.Start(ctx, &wg)

	// Wait for at least one tick.
	select {
	case <-repo.signal:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for worker tick")
	}

	cancel()

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not exit after context cancel")
	}

	if repo.calls.Load() < 1 {
		t.Fatalf("expected at least 1 call, got %d", repo.calls.Load())
	}
}

func TestLessonCompletionWorker_CancellationStopsTicks(t *testing.T) {
	repo := &fakeRepo{}
	w := NewLessonCompletionWorker(repo, 50*time.Millisecond, nil)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	w.Start(ctx, &wg)

	cancel()
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not exit promptly after cancel")
	}
	// No call should have happened (interval not yet elapsed).
	if repo.calls.Load() != 0 {
		t.Fatalf("expected 0 calls before first tick, got %d", repo.calls.Load())
	}
}

func TestLessonCompletionWorker_RepoErrorIsLoggedNotFatal(t *testing.T) {
	repo := &fakeRepo{err: errors.New("boom"), signal: make(chan struct{}, 4)}
	w := NewLessonCompletionWorker(repo, 10*time.Millisecond, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	w.Start(ctx, &wg)

	// Wait for two ticks to ensure the worker keeps running after an error.
	for i := 0; i < 2; i++ {
		select {
		case <-repo.signal:
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for tick %d", i+1)
		}
	}
	cancel()
	wg.Wait()
}
