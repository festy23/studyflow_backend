package worker

import (
	"context"
	"sync"
	"time"

	"common_library/logging"

	"go.uber.org/zap"
)

type CleanupService interface {
	CleanupOrphanUploads(ctx context.Context, olderThan time.Time) (int, error)
}

type OrphanCleanupWorker struct {
	service  CleanupService
	interval time.Duration
	ttl      time.Duration
	logger   *logging.Logger
}

func NewOrphanCleanupWorker(service CleanupService, interval, ttl time.Duration, logger *logging.Logger) *OrphanCleanupWorker {
	return &OrphanCleanupWorker{service: service, interval: interval, ttl: ttl, logger: logger}
}

func (w *OrphanCleanupWorker) Start(ctx context.Context, wg *sync.WaitGroup) {
	if w.interval <= 0 || w.ttl <= 0 {
		if w.logger != nil {
			w.logger.Warn(ctx, "orphan upload cleanup worker disabled")
		}
		return
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		w.process(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.process(ctx)
			}
		}
	}()
}

func (w *OrphanCleanupWorker) process(ctx context.Context) {
	deleted, err := w.service.CleanupOrphanUploads(ctx, time.Now().Add(-w.ttl))
	if err != nil {
		if w.logger != nil {
			w.logger.Error(ctx, "orphan upload cleanup failed", zap.Error(err))
		}
		return
	}
	if deleted > 0 && w.logger != nil {
		w.logger.Info(ctx, "orphan uploads cleaned", zap.Int("deleted", deleted))
	}
}
