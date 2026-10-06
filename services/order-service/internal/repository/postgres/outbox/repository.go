package outboxRepository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/postgres"
	txmanager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type dbOutboxEvent struct {
	ID            uuid.UUID
	CorrelationID uuid.UUID
	AggregateType string
	AggregateID   uuid.UUID
	EventType     string
	Payload       []byte
	CreatedAt     time.Time
	PublishedAt   *time.Time
}

func toDBOutboxEvent(event *domain.OutboxEvent) (*dbOutboxEvent, error) {
	payloadBytes, err := json.Marshal(event.Payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	return &dbOutboxEvent{
		ID:            event.ID,
		CorrelationID: event.CorrelationID,
		AggregateType: event.AggregateType,
		AggregateID:   event.AggregateID,
		EventType:     event.EventType,
		Payload:       payloadBytes,
		CreatedAt:     event.CreatedAt,
		PublishedAt:   event.PublishedAt,
	}, nil
}

func toDomainOutboxEvent(dbEvent *dbOutboxEvent) (*domain.OutboxEvent, error) {
	var payload map[string]any
	if err := json.Unmarshal(dbEvent.Payload, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}

	return &domain.OutboxEvent{
		ID:            dbEvent.ID,
		CorrelationID: dbEvent.CorrelationID,
		AggregateType: dbEvent.AggregateType,
		AggregateID:   dbEvent.AggregateID,
		EventType:     dbEvent.EventType,
		Payload:       payload,
		CreatedAt:     dbEvent.CreatedAt,
		PublishedAt:   dbEvent.PublishedAt,
	}, nil
}

type PostgresOutboxRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresOutboxRepository(pool *pgxpool.Pool) *PostgresOutboxRepository {
	return &PostgresOutboxRepository{pool: pool}
}

func (r *PostgresOutboxRepository) Create(ctx context.Context, event *domain.OutboxEvent) error {
	tx := txmanager.ExtractTx(ctx)
	querier := txmanager.GetQuerier(r.pool, tx)

	dbEvent, err := toDBOutboxEvent(event)
	if err != nil {
		return fmt.Errorf("convert to db event: %w", err)
	}

	query := `
		INSERT INTO outbox (id, correlation_id, aggregate_type, aggregate_id, event_type, payload, created_at)
		VALUES (@id, @correlation_id, @aggregate_type, @aggregate_id, @event_type, @payload, @created_at)
	`

	args := pgx.NamedArgs{
		"id":             dbEvent.ID,
		"correlation_id": dbEvent.CorrelationID,
		"aggregate_type": dbEvent.AggregateType,
		"aggregate_id":   dbEvent.AggregateID,
		"event_type":     dbEvent.EventType,
		"payload":        dbEvent.Payload,
		"created_at":     dbEvent.CreatedAt,
	}

	_, err = querier.Exec(ctx, query, args)
	if err != nil {
		if postgres.IsUniqueViolation(err) {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("insert outbox event: %w", err)
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
		dbEvent, err := scanOutboxEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("scan outbox event: %w", err)
		}

		domainEvent, err := toDomainOutboxEvent(dbEvent)
		if err != nil {
			return nil, fmt.Errorf("convert to domain event: %w", err)
		}

		events = append(events, domainEvent)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outbox events: %w", err)
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

	tag, err := querier.Exec(ctx, query, args)
	if err != nil {
		return fmt.Errorf("mark published: %w", err)
	}

	rowsAffected := tag.RowsAffected()

	if rowsAffected == 0 {
		return domain.ErrNotFound
	}

	if rowsAffected != int64(len(ids)) {
		return domain.ErrNotFound
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

func scanOutboxEvent(row pgx.Row) (*dbOutboxEvent, error) {
	var event dbOutboxEvent
	err := row.Scan(
		&event.ID,
		&event.CorrelationID,
		&event.AggregateType,
		&event.AggregateID,
		&event.EventType,
		&event.Payload,
		&event.CreatedAt,
		&event.PublishedAt,
	)
	if err != nil {
		return nil, err
	}
	return &event, nil
}
