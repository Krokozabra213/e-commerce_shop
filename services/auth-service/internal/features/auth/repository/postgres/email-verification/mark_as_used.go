package emailverificationRepo

import (
	"context"
	"fmt"
	"time"

	tx_manager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *PostgresEmailVerificationRepository) MarkAsUsed(ctx context.Context, tokenHash string) error {
	tx := tx_manager.ExtractTx(ctx)
	querier := tx_manager.GetQuerier(r.pool, tx)

	query := `
		UPDATE email_verification_tokens
		SET used_at = @used_at
		WHERE token_hash = @token_hash
	`

	args := pgx.NamedArgs{
		"token_hash": tokenHash,
		"used_at":    time.Now().UTC(),
	}

	ct, err := querier.Exec(ctx, query, args)
	if err != nil {
		return fmt.Errorf("mark email verification token as used: %w", err)
	}

	if ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}
