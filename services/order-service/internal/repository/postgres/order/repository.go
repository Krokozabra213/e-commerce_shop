package orderRepository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/postgres"
	tx_manager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresOrderRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresOrderRepository(pool *pgxpool.Pool) *PostgresOrderRepository {
	return &PostgresOrderRepository{pool: pool}
}

func (r *PostgresOrderRepository) Create(ctx context.Context, order *domain.Order) error {
	tx := tx_manager.ExtractTx(ctx)
	querier := tx_manager.GetQuerier(r.pool, tx)

	query := `
		INSERT INTO orders (
			id,
			user_id,
			idempotency_key,
			status,
			total_price,
			created_at,
			updated_at
		)
		VALUES (
			@id,
			@user_id,
			@idempotency_key,
			@status,
			@total_price,
			@created_at,
			@updated_at
		)
	`

	args := pgx.NamedArgs{
		"id":              order.ID,
		"user_id":         order.UserID,
		"idempotency_key": order.IdempotencyKey,
		"status":          order.Status.String(),
		"total_price":     order.TotalPrice,
		"created_at":      order.CreatedAt,
		"updated_at":      order.UpdatedAt,
	}

	_, err := querier.Exec(ctx, query, args)
	if err != nil {
		if postgres.IsUniqueViolation(err) {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("insert order: %w", err)
	}

	return nil
}

func (r *PostgresOrderRepository) CreateItems(ctx context.Context, items []domain.OrderItem) error {
	if len(items) == 0 {
		return nil
	}

	tx := tx_manager.ExtractTx(ctx)
	querier := tx_manager.GetQuerier(r.pool, tx)

	query := `
		INSERT INTO order_items (
			id,
			order_id,
			product_id,
			quantity,
			price,
			created_at
		)
		SELECT * FROM UNNEST(
			@ids::uuid[],
			@order_ids::uuid[],
			@product_ids::varchar[],
			@quantities::int[],
			@prices::bigint[],
			@created_ats::timestamptz[]
		)
	`

	ids := make([]uuid.UUID, len(items))
	orderIDs := make([]uuid.UUID, len(items))
	productIDs := make([]string, len(items))
	quantities := make([]int, len(items))
	prices := make([]int64, len(items))
	createdAts := make([]time.Time, len(items))

	for i, item := range items {
		ids[i] = item.ID
		orderIDs[i] = item.OrderID
		productIDs[i] = item.ProductID
		quantities[i] = item.Quantity
		prices[i] = item.Price
		createdAts[i] = item.CreatedAt
	}

	args := pgx.NamedArgs{
		"ids":         ids,
		"order_ids":   orderIDs,
		"product_ids": productIDs,
		"quantities":  quantities,
		"prices":      prices,
		"created_ats": createdAts,
	}

	_, err := querier.Exec(ctx, query, args)
	if err != nil {
		return fmt.Errorf("insert order items batch (unnest): %w", err)
	}

	return nil
}

func (r *PostgresOrderRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	tx := tx_manager.ExtractTx(ctx)
	querier := tx_manager.GetQuerier(r.pool, tx)

	query := `
		SELECT 
			id,
			user_id,
			idempotency_key,
			status,
			total_price,
			created_at,
			updated_at
		FROM orders
		WHERE id = @id
	`

	args := pgx.NamedArgs{"id": id}

	row := querier.QueryRow(ctx, query, args)

	order, err := scanOrder(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("scan order by id: %w", err)
	}

	return order, nil
}

func (r *PostgresOrderRepository) GetByIdempotencyKey(ctx context.Context, idempotencyKey uuid.UUID) (*domain.Order, error) {
	tx := tx_manager.ExtractTx(ctx)
	querier := tx_manager.GetQuerier(r.pool, tx)

	query := `
		SELECT 
			id,
			user_id,
			idempotency_key,
			status,
			total_price,
			created_at,
			updated_at
		FROM orders
		WHERE idempotency_key = @idempotency_key
	`

	args := pgx.NamedArgs{"idempotency_key": idempotencyKey}

	row := querier.QueryRow(ctx, query, args)

	order, err := scanOrder(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("scan order by idempotency_key: %w", err)
	}

	return order, nil
}

func (r *PostgresOrderRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.OrderStatus) error {
	tx := tx_manager.ExtractTx(ctx)
	querier := tx_manager.GetQuerier(r.pool, tx)

	query := `
		UPDATE orders
		SET 
			status = @status,
			updated_at = now()
		WHERE id = @id
	`

	args := pgx.NamedArgs{
		"id":     id,
		"status": status,
	}

	cmdTag, err := querier.Exec(ctx, query, args)
	if err != nil {
		return fmt.Errorf("update order status: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *PostgresOrderRepository) GetListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Order, error) {
	tx := tx_manager.ExtractTx(ctx)
	querier := tx_manager.GetQuerier(r.pool, tx)

	query := `
		SELECT 
			id,
			user_id,
			idempotency_key,
			status,
			total_price,
			created_at,
			updated_at
		FROM orders
		WHERE user_id = @user_id
		ORDER BY created_at DESC
	`

	args := pgx.NamedArgs{"user_id": userID}

	rows, err := querier.Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("query orders by user_id: %w", err)
	}
	defer rows.Close()

	orders := make([]domain.Order, 0)

	for rows.Next() {
		order, err := scanOrder(rows)
		if err != nil {
			return nil, fmt.Errorf("scan order row: %w", err)
		}
		orders = append(orders, *order)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return orders, nil
}

func (r *PostgresOrderRepository) UpdateStatusIfCurrent(
	ctx context.Context,
	id uuid.UUID,
	newStatus domain.OrderStatus,
	expectedStatus domain.OrderStatus,
) error {
	tx := tx_manager.ExtractTx(ctx)
	querier := tx_manager.GetQuerier(r.pool, tx)

	query := `
		UPDATE orders
		SET 
			status = @new_status,
			updated_at = now()
		WHERE id = @id AND status = @expected_status
	`

	args := pgx.NamedArgs{
		"id":              id,
		"new_status":      newStatus.String(),
		"expected_status": expectedStatus.String(),
	}

	cmdTag, err := querier.Exec(ctx, query, args)
	if err != nil {
		return fmt.Errorf("update order status conditionally: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return domain.ErrMismatchOrNotFound
	}

	return nil
}

func scanOrder(row pgx.Row) (*domain.Order, error) {
	var order domain.Order

	err := row.Scan(
		&order.ID,
		&order.UserID,
		&order.IdempotencyKey,
		&order.Status,
		&order.TotalPrice,
		&order.CreatedAt,
		&order.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &order, nil
}
