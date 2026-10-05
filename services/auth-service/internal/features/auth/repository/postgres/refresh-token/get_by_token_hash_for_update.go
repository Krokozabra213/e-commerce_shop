package refreshtokenRepo

import (
	"context"
	"errors"
	"fmt"

	tx_manager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *PostgresRefreshTokenRepository) GetByTokenHashForUpdate(
	ctx context.Context,
	tokenHash string,
) (*domain.RefreshToken, error) {
	tx := tx_manager.ExtractTx(ctx)
	querier := tx_manager.GetQuerier(r.pool, tx)

	query := `
		SELECT 
			id,
			user_id,
			token_hash,
			expires_at,
			revoked,
			created_at
		FROM refresh_tokens
		WHERE token_hash = @token_hash
		FOR UPDATE
	`

	args := pgx.NamedArgs{
		"token_hash": tokenHash,
	}

	row := querier.QueryRow(ctx, query, args)

	var token domain.RefreshToken
	err := row.Scan(
		&token.ID,
		&token.UserID,
		&token.TokenHash,
		&token.ExpiresAt,
		&token.Revoked,
		&token.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("scan refresh token for update: %w", err)
	}

	return &token, nil
}
