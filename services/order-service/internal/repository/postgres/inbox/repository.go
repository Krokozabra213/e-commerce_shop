package inboxRepository

import (
	"context"
	"fmt"

	"github.com/Krokozabra213/e-commerce_shop/infra/postgres"
	tx_manager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresInboxRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresInboxRepository(pool *pgxpool.Pool) *PostgresInboxRepository {
	return &PostgresInboxRepository{pool: pool}
}

func (r *PostgresInboxRepository) Create(ctx context.Context, event *domain.InboxEvent) error {
	tx := tx_manager.ExtractTx(ctx)
	querier := tx_manager.GetQuerier(r.pool, tx)

	query := `
		INSERT INTO inbox_events (
			id,
			event_id,
			correlation_id,
			event_type,
			payload,
			processed_at
		)
		VALUES (
			@id,
			@event_id,
			@correlation_id,
			@event_type,
			@payload,
			@processed_at
		)
	`

	args := pgx.NamedArgs{
		"id":             event.ID,
		"event_id":       event.EventID,
		"correlation_id": event.CorrelationID,
		"event_type":     event.EventType,
		"payload":        event.Payload,
		"processed_at":   event.ProcessedAt,
	}

	_, err := querier.Exec(ctx, query, args)
	if err != nil {
		if postgres.IsUniqueViolation(err) {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("insert inbox_event: %w", err)
	}

	return nil
}
