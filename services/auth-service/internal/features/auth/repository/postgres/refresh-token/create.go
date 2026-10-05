package refreshtokenRepo

import (
	"context"
	"fmt"

	"github.com/Krokozabra213/e-commerce_shop/infra/postgres"
	tx_manager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *PostgresRefreshTokenRepository) Create(ctx context.Context, token *domain.RefreshToken) error {
	tx := tx_manager.ExtractTx(ctx)
	querier := tx_manager.GetQuerier(r.pool, tx)

	query := `
		INSERT INTO refresh_tokens (
			id,
			user_id,
			token_hash,
			expires_at,
			revoked,
			created_at
		)
		VALUES (
			@id,
			@user_id,
			@token_hash,
			@expires_at,
			@revoked,
			@created_at
		)
	`

	args := pgx.NamedArgs{
		"id":         token.ID,
		"user_id":    token.UserID,
		"token_hash": token.TokenHash,
		"expires_at": token.ExpiresAt,
		"revoked":    token.Revoked,
		"created_at": token.CreatedAt,
	}

	_, err := querier.Exec(ctx, query, args)
	if err != nil {
		if postgres.IsUniqueViolation(err) {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("insert refresh token: %w", err)
	}

	return nil
}
