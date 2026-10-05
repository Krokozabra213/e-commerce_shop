package outboxRepo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	txmanager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresOutboxRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresOutboxRepository(pool *pgxpool.Pool) *PostgresOutboxRepository {
	return &PostgresOutboxRepository{
		pool: pool,
	}
}

func (r *PostgresOutboxRepository) Create(ctx context.Context, event *domain.OutboxEvent) error {
	tx := txmanager.ExtractTx(ctx)
	querier := txmanager.GetQuerier(r.pool, tx)

	payloadJSON, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	query := `
		INSERT INTO outbox (
			id,
			aggregate_type,
			aggregate_id,
			event_type,
			payload,
			created_at
		)
		VALUES (
			@id,
			@aggregate_type,
			@aggregate_id,
			@event_type,
			@payload,
			@created_at
		)
	`

	args := pgx.NamedArgs{
		"id":             event.ID,
		"aggregate_type": event.AggregateType,
		"aggregate_id":   event.AggregateID,
		"event_type":     event.EventType,
		"payload":        payloadJSON,
		"created_at":     event.CreatedAt,
	}

	_, err = querier.Exec(ctx, query, args)
	if err != nil {
		return fmt.Errorf("execute query: %w", err)
	}

	return nil
}

func (r *PostgresOutboxRepository) ClaimBatch(
	ctx context.Context,
	eventType string,
	batchSize int,
	workerID string,
	lease time.Duration,
) ([]*domain.OutboxEvent, error) {
	tx := txmanager.ExtractTx(ctx)
	querier := txmanager.GetQuerier(r.pool, tx)

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
			ORDER BY created_at
			LIMIT @batch_size
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, aggregate_type, aggregate_id, event_type, payload, created_at
	`

	args := pgx.NamedArgs{
		"lease":      lease,
		"worker_id":  workerID,
		"batch_size": batchSize,
		"event_type": eventType,
	}

	rows, err := querier.Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("claim batch: %w", err)
	}
	defer rows.Close()

	var events []*domain.OutboxEvent
	for rows.Next() {
		event, err := scanOutboxEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outbox rows: %w", err)
	}

	return events, nil
}

func (r *PostgresOutboxRepository) MarkPublished(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}

	tx := txmanager.ExtractTx(ctx)
	querier := txmanager.GetQuerier(r.pool, tx)

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

	_, err := querier.Exec(ctx, query, args)
	if err != nil {
		return fmt.Errorf("mark published: %w", err)
	}

	return nil
}

func (r *PostgresOutboxRepository) ReleaseLocks(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}

	tx := txmanager.ExtractTx(ctx)
	querier := txmanager.GetQuerier(r.pool, tx)

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

func scanOutboxEvent(rows pgx.Rows) (*domain.OutboxEvent, error) {
	var event domain.OutboxEvent
	var payloadJSON []byte

	err := rows.Scan(
		&event.ID,
		&event.AggregateType,
		&event.AggregateID,
		&event.EventType,
		&payloadJSON,
		&event.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan outbox event: %w", err)
	}

	if err := json.Unmarshal(payloadJSON, &event.Payload); err != nil {
		return nil, fmt.Errorf("unmarshal outbox payload: %w", err)
	}

	return &event, nil
}
