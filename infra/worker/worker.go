package worker

import (
	"context"
	"log/slog"
	"math/rand"
	"time"
)

type BatchProcessor interface {
	ProcessBatch(ctx context.Context) (int, error)
	WorkerID() string
}

type Worker struct {
	logger       *slog.Logger
	processor    BatchProcessor
	pollInterval time.Duration
	errorBackoff time.Duration
}

func NewWorker(
	logger *slog.Logger,
	processor BatchProcessor,
	pollInterval time.Duration,
	errorBackoff time.Duration,
) *Worker {
	return &Worker{
		logger: logger.With(
			"component", "outbox_worker",
			"worker_id", processor.WorkerID(),
		),
		processor:    processor,
		pollInterval: pollInterval,
		errorBackoff: errorBackoff,
	}
}

func (w *Worker) Run(ctx context.Context) {
	w.logger.Info("outbox worker started",
		"poll_interval", w.pollInterval,
		"error_backoff", w.errorBackoff,
	)

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("outbox worker stopping...")
			return
		default:
			processedCount, err := w.processor.ProcessBatch(ctx)
			if err != nil {
				w.logger.Error("failed to process outbox batch", "error", err)
				w.sleepWithJitter(ctx, w.errorBackoff)
				continue
			}

			if processedCount > 0 {
				continue
			}

			w.sleepWithJitter(ctx, w.pollInterval)
		}
	}
}

func (w *Worker) sleepWithJitter(ctx context.Context, base time.Duration) {
	if base <= 0 {
		return
	}

	jitterPercent := 0.15
	maxJitter := float64(base) * jitterPercent

	randomMultiplier := (rand.Float64() * 2) - 1
	jitter := time.Duration(randomMultiplier * maxJitter)

	finalDuration := base + jitter

	timer := time.NewTimer(finalDuration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
