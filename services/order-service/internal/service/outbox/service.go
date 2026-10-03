package outboxService

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	infracfg "github.com/Krokozabra213/e-commerce_shop/infra/config"
	"github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/domain"
	"github.com/google/uuid"
)

type OutboxRepository interface {
	FetchUnpublishedByEventType(
		ctx context.Context,
		eventType string,
		limit int,
		workerID string,
		lease time.Duration,
	) ([]*domain.OutboxEvent, error)

	MarkPublished(ctx context.Context, ids []uuid.UUID) error

	ReleaseLocks(ctx context.Context, ids []uuid.UUID) error
}

type EventPublisher interface {
	Publish(ctx context.Context, event *domain.OutboxEvent) error
}

type Service struct {
	repo         OutboxRepository
	publisher    EventPublisher
	logger       *slog.Logger
	workerID     string
	outboxConfig infracfg.OutboxConfig
}

func New(
	repo OutboxRepository,
	publisher EventPublisher,
	logger *slog.Logger,
	outboxConfig infracfg.OutboxConfig,
) *Service {
	workerID := uuid.NewString()

	logger = logger.With(
		slog.String("component", "outbox_service"),
		slog.String("worker_id", workerID),
	)
	return &Service{
		repo:         repo,
		publisher:    publisher,
		logger:       logger,
		workerID:     workerID,
		outboxConfig: outboxConfig,
	}
}

func (s *Service) ProcessBatch(ctx context.Context, eventType string) (int, error) {
	events, err := s.repo.FetchUnpublishedByEventType(ctx, eventType, s.outboxConfig.BatchSize, s.workerID, s.outboxConfig.Lease)
	if err != nil {
		return 0, fmt.Errorf("claim batch: %w", err)
	}
	if len(events) == 0 {
		return 0, nil
	}

	s.logger.Debug("claimed outbox batch", "count", len(events))

	var (
		publishedIDs = make([]uuid.UUID, 0, len(events))
		failedIDs    = make([]uuid.UUID, 0, len(events))
	)

	for _, event := range events {
		if ctx.Err() != nil {
			failedIDs = append(failedIDs, remainingIDs(events, publishedIDs, failedIDs)...)
			break
		}

		sendCtx, cancel := context.WithTimeout(ctx, s.outboxConfig.SendTimeout)
		err := s.publisher.Publish(sendCtx, event)
		cancel()

		if err != nil {
			s.logger.Warn("failed to publish event",
				"outbox_id", event.ID,
				"event_type", event.EventType,
				"aggregate_id", event.AggregateID,
				"error", err,
			)
			failedIDs = append(failedIDs, event.ID)
			continue
		}

		publishedIDs = append(publishedIDs, event.ID)
	}

	if len(publishedIDs) > 0 {
		if err := s.repo.MarkPublished(ctx, publishedIDs); err != nil {
			s.logger.Error("failed to mark events as published",
				"count", len(publishedIDs),
				"error", err,
			)
		}
	}

	if len(failedIDs) > 0 {
		if err := s.repo.ReleaseLocks(ctx, failedIDs); err != nil {
			s.logger.Error("failed to release locks",
				"count", len(failedIDs),
				"error", err,
			)
		}
	}

	s.logger.Debug("batch processed",
		"total", len(events),
		"published", len(publishedIDs),
		"failed", len(failedIDs),
	)

	return len(events), nil
}

func (s *Service) WorkerID() string {
	return s.workerID
}

func remainingIDs(events []*domain.OutboxEvent, published, failed []uuid.UUID) []uuid.UUID {
	processed := make(map[uuid.UUID]struct{}, len(published)+len(failed))
	for _, id := range published {
		processed[id] = struct{}{}
	}
	for _, id := range failed {
		processed[id] = struct{}{}
	}

	var remaining []uuid.UUID
	for _, e := range events {
		if _, ok := processed[e.ID]; !ok {
			remaining = append(remaining, e.ID)
		}
	}
	return remaining
}
