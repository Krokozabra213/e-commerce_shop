package emailverificationRepo

import (
	"context"
	"fmt"

	"github.com/Krokozabra213/e-commerce_shop/infra/postgres"
	txmanager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *PostgresEmailVerificationRepository) Create(ctx context.Context, token *domain.EmailVerificationToken) error {
	tx := txmanager.ExtractTx(ctx)
	querier := txmanager.GetQuerier(r.pool, tx)

	query := `
		INSERT INTO email_verification_tokens (
			id,
			user_id,
			token_hash,
			expires_at,
			created_at,
			used_at
		)
		VALUES (
			@id,
			@user_id,
			@token_hash,
			@expires_at,
			@created_at,
			@used_at
		)
	`

	args := pgx.NamedArgs{
		"id":         token.ID,
		"user_id":    token.UserID,
		"token_hash": token.TokenHash,
		"expires_at": token.ExpiresAt,
		"created_at": token.CreatedAt,
		"used_at":    token.UsedAt,
	}

	_, err := querier.Exec(ctx, query, args)
	if err != nil {
		if postgres.IsUniqueViolation(err) {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("insert email verification token: %w", err)
	}

	return nil
}
