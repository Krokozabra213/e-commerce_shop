package userRepo

import (
	"context"
	"errors"
	"fmt"

	tx_manager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *PostgresUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	tx := tx_manager.ExtractTx(ctx)
	querier := tx_manager.GetQuerier(r.pool, tx)

	query := "SELECT id, email, password_hash, email_confirmed_at, created_at, updated_at, deleted_at FROM users WHERE id = @id"

	args := pgx.NamedArgs{
		"id": id,
	}

	row := querier.QueryRow(ctx, query, args)

	var user domain.User
	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.EmailConfirmedAt,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.NotFoundError
		}
		return nil, fmt.Errorf("scan user by id: %w", err)
	}

	return &user, nil
}
