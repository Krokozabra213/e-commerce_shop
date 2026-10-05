package outboxRepository

import (
	"context"
	"fmt"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/postgres"
	tx_manager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresOutboxRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresOutboxRepository(pool *pgxpool.Pool) *PostgresOutboxRepository {
	return &PostgresOutboxRepository{pool: pool}
}

func (r *PostgresOutboxRepository) Create(ctx context.Context, event *domain.OutboxEvent) error {
	tx := tx_manager.ExtractTx(ctx)
	querier := tx_manager.GetQuerier(r.pool, tx)

	query := `
		INSERT INTO outbox (
			id,
			correlation_id,
			aggregate_type,
			aggregate_id,
			event_type,
			payload,
			created_at
		)
		VALUES (
			@id,
			@correlation_id,
			@aggregate_type,
			@aggregate_id,
			@event_type,
			@payload,
			@created_at
		)
	`

	args := pgx.NamedArgs{
		"id":             event.ID,
		"correlation_id": event.CorrelationID,
		"aggregate_type": event.AggregateType,
		"aggregate_id":   event.AggregateID,
		"event_type":     event.EventType,
		"payload":        event.Payload,
		"created_at":     event.CreatedAt,
	}

	_, err := querier.Exec(ctx, query, args)
	if err != nil {
		if postgres.IsUniqueViolation(err) {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("insert outbox event: %w", err)
	}

	return nil
}

func (r *PostgresOutboxRepository) ReleaseLocks(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}

	tx := tx_manager.ExtractTx(ctx)
	querier := tx_manager.GetQuerier(r.pool, tx)

	query := `
		UPDATE outbox
		SET locked_until = NULL,
		    locked_by    = NULL
		WHERE id = ANY(@ids)
		  AND published_at IS NULL
	`

	args := pgx.NamedArgs{
		"ids": ids,
	}

	_, err := querier.Exec(ctx, query, args)
	if err != nil {
		return fmt.Errorf("release locks: %w", err)
	}

	return nil
}

func (r *PostgresOutboxRepository) MarkPublished(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}

	tx := tx_manager.ExtractTx(ctx)
	querier := tx_manager.GetQuerier(r.pool, tx)

	query := `
		UPDATE outbox
		SET published_at = now(),
		    locked_until = NULL,
		    locked_by    = NULL
		WHERE id = ANY(@ids)
		  AND published_at IS NULL
	`

	args := pgx.NamedArgs{
		"ids": ids,
	}

	tag, err := querier.Exec(ctx, query, args)
	if err != nil {
		return fmt.Errorf("mark published: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	if tag.RowsAffected() != int64(len(ids)) {
		return domain.ErrNotFound
	}

	return nil
}

func (r *PostgresOutboxRepository) FetchUnpublishedByEventType(
	ctx context.Context,
	eventType string,
	limit int,
	workerID string,
	lease time.Duration,
) ([]*domain.OutboxEvent, error) {
	tx := tx_manager.ExtractTx(ctx)
	querier := tx_manager.GetQuerier(r.pool, tx)

	query := `
		UPDATE outbox
		SET locked_until = now() + @lease,
		    locked_by    = @worker_id
		WHERE id IN (
			SELECT id
			FROM outbox
			WHERE published_at IS NULL
			  AND event_type = @event_type
			  AND (locked_until IS NULL OR locked_until < now())
			ORDER BY created_at ASC
			LIMIT @limit
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, correlation_id, aggregate_type, aggregate_id, event_type, payload, created_at, published_at
	`

	args := pgx.NamedArgs{
		"lease":      lease,
		"worker_id":  workerID,
		"event_type": eventType,
		"limit":      limit,
	}

	rows, err := querier.Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("fetch unpublished outbox events: %w", err)
	}
	defer rows.Close()

	events := make([]*domain.OutboxEvent, 0, limit)
	for rows.Next() {
		event, err := scanOutboxEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("scan outbox event: %w", err)
		}
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outbox events: %w", err)
	}

	return events, nil
}

func scanOutboxEvent(row pgx.Row) (*domain.OutboxEvent, error) {
	var e domain.OutboxEvent
	err := row.Scan(
		&e.ID,
		&e.CorrelationID,
		&e.AggregateType,
		&e.AggregateID,
		&e.EventType,
		&e.Payload,
		&e.CreatedAt,
		&e.PublishedAt,
	)
	if err != nil {
		return nil, err
	}
	return &e, nil
}
