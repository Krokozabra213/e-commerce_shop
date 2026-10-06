package refreshtokenRepo

import (
	"context"
	"fmt"

	txmanager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *PostgresRefreshTokenRepository) Revoke(ctx context.Context, tokenHash string) error {
	tx := txmanager.ExtractTx(ctx)
	querier := txmanager.GetQuerier(r.pool, tx)

	query := `
		UPDATE refresh_tokens
		SET revoked = TRUE
		WHERE token_hash = @token_hash
	`

	args := pgx.NamedArgs{
		"token_hash": tokenHash,
	}

	ct, err := querier.Exec(ctx, query, args)
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}

	if ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}
