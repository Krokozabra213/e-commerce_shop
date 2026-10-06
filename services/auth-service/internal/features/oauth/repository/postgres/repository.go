package oauthrepo

import (
	"context"
	"errors"
	"fmt"

	"github.com/Krokozabra213/e-commerce_shop/infra/postgres"
	txmanager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresOAuthRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresOAuthRepository(pool *pgxpool.Pool) *PostgresOAuthRepository {
	return &PostgresOAuthRepository{pool: pool}
}

func (r *PostgresOAuthRepository) Create(ctx context.Context, account *domain.OauthAccount) error {
	tx := txmanager.ExtractTx(ctx)
	querier := txmanager.GetQuerier(r.pool, tx)

	query := `
		INSERT INTO oauth_accounts (
			id,
			user_id,
			provider,
			provider_user_id,
			created_at
		)
		VALUES (
			@id,
			@user_id,
			@provider,
			@provider_user_id,
			@created_at
		)
	`

	args := pgx.NamedArgs{
		"id":               account.ID,
		"user_id":          account.UserID,
		"provider":         account.Provider,
		"provider_user_id": account.ProviderUserID,
		"created_at":       account.CreatedAt,
	}

	_, err := querier.Exec(ctx, query, args)
	if err != nil {
		if postgres.IsUniqueViolation(err) {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("execute query: %w", err)
	}

	return nil
}

func (r *PostgresOAuthRepository) GetByProviderAndProviderUserID(
	ctx context.Context,
	provider domain.OAuthProvider,
	providerUserID string,
) (*domain.OauthAccount, error) {
	tx := txmanager.ExtractTx(ctx)
	querier := txmanager.GetQuerier(r.pool, tx)

	query := `
		SELECT 
			id,
			user_id,
			provider,
			provider_user_id,
			created_at
		FROM oauth_accounts
		WHERE provider = @provider
		  AND provider_user_id = @provider_user_id
	`

	args := pgx.NamedArgs{
		"provider":         provider.String(),
		"provider_user_id": providerUserID,
	}

	row := querier.QueryRow(ctx, query, args)

	var account domain.OauthAccount
	err := row.Scan(
		&account.ID,
		&account.UserID,
		&account.Provider,
		&account.ProviderUserID,
		&account.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("scan row: %w", err)
	}

	return &account, nil
}
