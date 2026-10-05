package userRepo

import (
	"context"
	"fmt"
	"time"

	txmanager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *PostgresUserRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	tx := txmanager.ExtractTx(ctx)
	querier := txmanager.GetQuerier(r.pool, tx)

	query := `
		UPDATE users
		SET deleted_at = @deleted_at, updated_at = @updated_at
		WHERE id = @id AND deleted_at IS NULL
	`

	now := time.Now()
	args := pgx.NamedArgs{
		"id":         id,
		"deleted_at": now,
		"updated_at": now,
	}

	tag, err := querier.Exec(ctx, query, args)
	if err != nil {
		return fmt.Errorf("soft delete user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}
