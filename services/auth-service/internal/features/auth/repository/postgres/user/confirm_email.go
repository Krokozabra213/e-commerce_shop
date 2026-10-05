package userRepo

import (
	"context"
	"fmt"
	"time"

	tx_manager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *PostgresUserRepository) ConfirmEmail(ctx context.Context, userID uuid.UUID) error {
	tx := tx_manager.ExtractTx(ctx)
	querier := tx_manager.GetQuerier(r.pool, tx)

	query := `
		UPDATE users
		SET 
			email_confirmed_at = @now,
			updated_at = @now
		WHERE id = @id
	`

	args := pgx.NamedArgs{
		"id":  userID,
		"now": time.Now().UTC(),
	}

	ct, err := querier.Exec(ctx, query, args)
	if err != nil {
		return fmt.Errorf("execute confirm email query: %w", err)
	}

	if ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}
