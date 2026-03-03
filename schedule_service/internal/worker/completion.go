package worker

import (
	"context"
	"sync"
	"time"

	"common_library/logging"

	"go.uber.org/zap"
)

// CompletionRepository is the minimal repository surface the worker needs.
// Defined as an interface here to keep the worker testable without pulling in
// the full repo.Repository.
type CompletionRepository interface {
	UpdateCompletedLessons(ctx context.Context) (int, error)
}

// LessonCompletionWorker periodically marks past 'booked' lessons as
// 'completed' by invoking CompletionRepository.UpdateCompletedLessons.
type LessonCompletionWorker struct {
	repo     CompletionRepository
	interval time.Duration
	logger   *logging.Logger
	// tickTimeout bounds a single UpdateCompletedLessons call.
	tickTimeout time.Duration
}

// NewLessonCompletionWorker constructs a worker. interval must be > 0.
func NewLessonCompletionWorker(repo CompletionRepository, interval time.Duration, logger *logging.Logger) *LessonCompletionWorker {
	return &LessonCompletionWorker{
		repo:        repo,
		interval:    interval,
		logger:      logger,
		tickTimeout: 30 * time.Second,
	}
}

// Run blocks until ctx is cancelled. It does not run an initial tick: the
// first invocation occurs after `interval` has elapsed. Use this from a
// goroutine and use a sync.WaitGroup for graceful shutdown coordination.
func (w *LessonCompletionWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *LessonCompletionWorker) runOnce(parent context.Context) {
	tickCtx, cancel := context.WithTimeout(parent, w.tickTimeout)
	defer cancel()

	updated, err := w.repo.UpdateCompletedLessons(tickCtx)
	if err != nil {
		if w.logger != nil {
			w.logger.Error(parent, "lesson completion worker tick failed", zap.Error(err))
		}
		return
	}
	if w.logger != nil && updated > 0 {
		w.logger.Info(parent, "lesson completion worker updated lessons", zap.Int("updated", updated))
	}
}

// Start launches Run on a goroutine and registers it with wg. Returns
// immediately.
func (w *LessonCompletionWorker) Start(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		w.Run(ctx)
	}()
}
