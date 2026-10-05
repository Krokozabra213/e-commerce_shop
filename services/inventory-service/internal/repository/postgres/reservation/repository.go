package reservationRepository

import (
	"context"
	"fmt"

	txmanager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresReservationRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresReservationRepository(pool *pgxpool.Pool) *PostgresReservationRepository {
	return &PostgresReservationRepository{pool: pool}
}

func (r *PostgresReservationRepository) CreateBatch(
	ctx context.Context,
	reservations []*domain.Reservation,
) error {
	if len(reservations) == 0 {
		return nil
	}

	tx := txmanager.ExtractTx(ctx)
	querier := txmanager.GetQuerier(r.pool, tx)

	query := `
		INSERT INTO reservations (
			id, order_id, product_id, quantity, status
		)
		SELECT * FROM UNNEST(
			@ids::uuid[],
			@order_ids::uuid[],
			@product_ids::varchar[],
			@quantities::int[],
			@statuses::text[]
		)
	`

	ids := make([]uuid.UUID, len(reservations))
	orderIDs := make([]uuid.UUID, len(reservations))
	productIDs := make([]string, len(reservations))
	quantities := make([]int, len(reservations))
	statuses := make([]string, len(reservations))

	for i, res := range reservations {
		ids[i] = res.ID
		orderIDs[i] = res.OrderID
		productIDs[i] = res.ProductID
		quantities[i] = res.Quantity
		statuses[i] = string(res.Status)
	}

	args := pgx.NamedArgs{
		"ids":         ids,
		"order_ids":   orderIDs,
		"product_ids": productIDs,
		"quantities":  quantities,
		"statuses":    statuses,
	}

	_, err := querier.Exec(ctx, query, args)
	if err != nil {
		return fmt.Errorf("insert reservations batch: %w", err)
	}

	return nil
}

func (r *PostgresReservationRepository) GetActiveByOrderID(
	ctx context.Context,
	orderID uuid.UUID,
) ([]*domain.Reservation, error) {
	tx := txmanager.ExtractTx(ctx)
	querier := txmanager.GetQuerier(r.pool, tx)

	query := `
		SELECT 
			id, order_id, product_id, quantity, status, created_at, released_at
		FROM reservations
		WHERE order_id = @order_id AND status = 'ACTIVE'
		ORDER BY product_id
		FOR UPDATE
	`

	args := pgx.NamedArgs{"order_id": orderID}

	rows, err := querier.Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("query active reservations by order_id: %w", err)
	}
	defer rows.Close()

	reservations := make([]*domain.Reservation, 0)
	for rows.Next() {
		res, err := scanReservation(rows)
		if err != nil {
			return nil, fmt.Errorf("scan reservation: %w", err)
		}
		reservations = append(reservations, res)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate reservations: %w", err)
	}

	return reservations, nil
}

func (r *PostgresReservationRepository) ReleaseByOrderID(
	ctx context.Context,
	orderID uuid.UUID,
) (int, error) {
	tx := txmanager.ExtractTx(ctx)
	querier := txmanager.GetQuerier(r.pool, tx)

	query := `
		UPDATE reservations
		SET 
			status = 'RELEASED',
			released_at = now()
		WHERE order_id = @order_id AND status = 'ACTIVE'
	`

	args := pgx.NamedArgs{"order_id": orderID}

	tag, err := querier.Exec(ctx, query, args)
	if err != nil {
		return 0, fmt.Errorf("release reservations by order_id: %w", err)
	}

	return int(tag.RowsAffected()), nil
}

func scanReservation(row pgx.Row) (*domain.Reservation, error) {
	var r domain.Reservation
	var status string

	err := row.Scan(
		&r.ID,
		&r.OrderID,
		&r.ProductID,
		&r.Quantity,
		&status,
		&r.CreatedAt,
		&r.ReleasedAt,
	)
	if err != nil {
		return nil, err
	}

	r.Status = domain.ReservationStatus(status)
	return &r, nil
}

func (r *PostgresReservationRepository) ExistsByOrderID(
	ctx context.Context,
	orderID uuid.UUID,
) (bool, error) {
	tx := txmanager.ExtractTx(ctx)
	querier := txmanager.GetQuerier(r.pool, tx)

	query := `
		SELECT EXISTS(
			SELECT 1 
			FROM reservations 
			WHERE order_id = @order_id
		)
	`

	args := pgx.NamedArgs{"order_id": orderID}

	var exists bool
	if err := querier.QueryRow(ctx, query, args).Scan(&exists); err != nil {
		return false, fmt.Errorf("check reservation exists: %w", err)
	}

	return exists, nil
}
