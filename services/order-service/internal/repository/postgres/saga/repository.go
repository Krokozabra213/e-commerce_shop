package sagaRepository

import (
	"context"
	"errors"
	"fmt"

	"github.com/Krokozabra213/e-commerce_shop/infra/postgres"
	tx_manager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresSagaRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresSagaRepository(pool *pgxpool.Pool) *PostgresSagaRepository {
	return &PostgresSagaRepository{pool: pool}
}

func (r *PostgresSagaRepository) Create(ctx context.Context, saga *domain.SagaState) error {
	tx := tx_manager.ExtractTx(ctx)
	querier := tx_manager.GetQuerier(r.pool, tx)

	query := `
		INSERT INTO saga_state (
			order_id,
			correlation_id,
			current_step,
			status,
			created_at,
			updated_at
		)
		VALUES (
			@order_id,
			@correlation_id,
			@current_step,
			@status,
			@created_at,
			@updated_at
		)
	`

	args := pgx.NamedArgs{
		"order_id":       saga.OrderID,
		"correlation_id": saga.CorrelationID,
		"current_step":   saga.CurrentStep,
		"status":         saga.Status,
		"created_at":     saga.CreatedAt,
		"updated_at":     saga.UpdatedAt,
	}

	_, err := querier.Exec(ctx, query, args)
	if err != nil {
		if postgres.IsUniqueViolation(err) {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("insert saga_state: %w", err)
	}

	return nil
}

func (r *PostgresSagaRepository) GetByOrderID(ctx context.Context, orderID uuid.UUID) (*domain.SagaState, error) {
	tx := tx_manager.ExtractTx(ctx)
	querier := tx_manager.GetQuerier(r.pool, tx)

	query := `
		SELECT 
			order_id,
			correlation_id,
			current_step,
			status,
			created_at,
			updated_at
		FROM saga_state
		WHERE order_id = @order_id
	`

	args := pgx.NamedArgs{"order_id": orderID}

	row := querier.QueryRow(ctx, query, args)

	saga, err := scanSaga(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("scan saga by order_id: %w", err)
	}

	return saga, nil
}

func (r *PostgresSagaRepository) Update(ctx context.Context, saga *domain.SagaState) error {
	tx := tx_manager.ExtractTx(ctx)
	querier := tx_manager.GetQuerier(r.pool, tx)

	query := `
		UPDATE saga_state
		SET 
			current_step = @current_step,
			status = @status,
			updated_at = @updated_at
		WHERE order_id = @order_id
	`

	args := pgx.NamedArgs{
		"order_id":     saga.OrderID,
		"current_step": saga.CurrentStep,
		"status":       saga.Status,
		"updated_at":   saga.UpdatedAt,
	}

	cmdTag, err := querier.Exec(ctx, query, args)
	if err != nil {
		return fmt.Errorf("update saga_state: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func scanSaga(row pgx.Row) (*domain.SagaState, error) {
	var saga domain.SagaState

	err := row.Scan(
		&saga.OrderID,
		&saga.CorrelationID,
		&saga.CurrentStep,
		&saga.Status,
		&saga.CreatedAt,
		&saga.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &saga, nil
}
