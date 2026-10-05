package stockRepository

import (
	"context"
	"errors"
	"fmt"

	"github.com/Krokozabra213/e-commerce_shop/infra/postgres"
	tx_manager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStockRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresStockRepository(pool *pgxpool.Pool) *PostgresStockRepository {
	return &PostgresStockRepository{pool: pool}
}

func (r *PostgresStockRepository) Create(ctx context.Context, stock *domain.Stock) error {
	tx := tx_manager.ExtractTx(ctx)
	querier := tx_manager.GetQuerier(r.pool, tx)

	query := `
		INSERT INTO stocks (
			product_id,
			available_quantity,
			created_at,
			updated_at
		)
		VALUES (
			@product_id,
			@available_quantity,
			@created_at,
			@updated_at
		)
	`

	args := pgx.NamedArgs{
		"product_id":         stock.ProductID,
		"available_quantity": stock.AvailableQuantity,
		"created_at":         stock.CreatedAt,
		"updated_at":         stock.UpdatedAt,
	}

	_, err := querier.Exec(ctx, query, args)
	if err != nil {
		if postgres.IsUniqueViolation(err) {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("insert stock: %w", err)
	}

	return nil
}

func (r *PostgresStockRepository) GetByProductID(ctx context.Context, productID string) (*domain.Stock, error) {
	tx := tx_manager.ExtractTx(ctx)
	querier := tx_manager.GetQuerier(r.pool, tx)

	query := `
		SELECT 
			product_id, available_quantity, created_at, updated_at
		FROM stocks
		WHERE product_id = @product_id
	`

	args := pgx.NamedArgs{"product_id": productID}

	row := querier.QueryRow(ctx, query, args)

	stock, err := scanStock(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("scan stock by product_id: %w", err)
	}

	return stock, nil
}

func (r *PostgresStockRepository) GetByProductIDs(
	ctx context.Context,
	productIDs []string,
) (map[string]int, error) {
	if len(productIDs) == 0 {
		return map[string]int{}, nil
	}

	tx := tx_manager.ExtractTx(ctx)
	querier := tx_manager.GetQuerier(r.pool, tx)

	query := `
		SELECT product_id, available_quantity
		FROM stocks
		WHERE product_id = ANY(@product_ids)
	`

	args := pgx.NamedArgs{"product_ids": productIDs}

	rows, err := querier.Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("query stocks by product_ids: %w", err)
	}
	defer rows.Close()

	result := make(map[string]int, len(productIDs))
	for rows.Next() {
		var productID string
		var quantity int
		if err := rows.Scan(&productID, &quantity); err != nil {
			return nil, fmt.Errorf("scan stock: %w", err)
		}
		result[productID] = quantity
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stocks: %w", err)
	}

	return result, nil
}

func (r *PostgresStockRepository) DecreaseQuantity(
	ctx context.Context,
	productID string,
	quantity int,
) error {
	tx := tx_manager.ExtractTx(ctx)
	querier := tx_manager.GetQuerier(r.pool, tx)

	query := `
		UPDATE stocks
		SET 
			available_quantity = available_quantity - @quantity,
			updated_at = now()
		WHERE product_id = @product_id 
		  AND available_quantity >= @quantity
	`

	args := pgx.NamedArgs{
		"product_id": productID,
		"quantity":   quantity,
	}

	tag, err := querier.Exec(ctx, query, args)
	if err != nil {
		return fmt.Errorf("decrease stock quantity: %w", err)
	}

	if tag.RowsAffected() == 0 {
		checkQuery := `SELECT EXISTS(SELECT 1 FROM stocks WHERE product_id = @product_id)`
		checkArgs := pgx.NamedArgs{
			"product_id": productID,
		}

		var exists bool
		if err := querier.QueryRow(ctx, checkQuery, checkArgs).Scan(&exists); err != nil {
			return fmt.Errorf("check product exists: %w", err)
		}

		if !exists {
			return domain.ErrNotFound
		}

		return domain.ErrInsufficientStock
	}

	return nil
}

func (r *PostgresStockRepository) IncreaseQuantity(
	ctx context.Context,
	productID string,
	quantity int,
) error {
	tx := tx_manager.ExtractTx(ctx)
	querier := tx_manager.GetQuerier(r.pool, tx)

	query := `
		UPDATE stocks
		SET 
			available_quantity = available_quantity + @quantity,
			updated_at = now()
		WHERE product_id = @product_id
	`

	args := pgx.NamedArgs{
		"product_id": productID,
		"quantity":   quantity,
	}

	tag, err := querier.Exec(ctx, query, args)
	if err != nil {
		return fmt.Errorf("increase stock quantity: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func scanStock(row pgx.Row) (*domain.Stock, error) {
	var s domain.Stock
	err := row.Scan(
		&s.ProductID,
		&s.AvailableQuantity,
		&s.CreatedAt,
		&s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
