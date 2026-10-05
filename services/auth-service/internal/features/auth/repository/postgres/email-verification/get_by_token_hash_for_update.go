package emailverificationRepo

import (
	"context"
	"errors"
	"fmt"

	txmanager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *PostgresEmailVerificationRepository) GetByTokenHashForUpdate(
	ctx context.Context,
	tokenHash string,
) (*domain.EmailVerificationToken, error) {
	tx := txmanager.ExtractTx(ctx)
	querier := txmanager.GetQuerier(r.pool, tx)

	query := `
		SELECT 
			id,
			user_id,
			token_hash,
			expires_at,
			created_at,
			used_at
		FROM email_verification_tokens
		WHERE token_hash = @token_hash
		FOR UPDATE
	`

	args := pgx.NamedArgs{
		"token_hash": tokenHash,
	}

	row := querier.QueryRow(ctx, query, args)

	var token domain.EmailVerificationToken
	err := row.Scan(
		&token.ID,
		&token.UserID,
		&token.TokenHash,
		&token.ExpiresAt,
		&token.CreatedAt,
		&token.UsedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("scan email verification token for update: %w", err)
	}

	return &token, nil
}
