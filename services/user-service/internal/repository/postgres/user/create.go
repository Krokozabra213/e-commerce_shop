package userRepo

import (
	"context"
	"fmt"

	"github.com/Krokozabra213/e-commerce_shop/infra/postgres"
	tx_manager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *PostgresUserRepository) Create(ctx context.Context, user *domain.User) error {
	tx := tx_manager.ExtractTx(ctx)
	querier := tx_manager.GetQuerier(r.pool, tx)

	query := `
		INSERT INTO users (
			id,
			email,
			first_name,
			last_name,
			phone,
			avatar_url,
			created_at,
			updated_at
		)
		VALUES (
			@id,
			@email,
			@first_name,
			@last_name,
			@phone,
			@avatar_url,
			@created_at,
			@updated_at
		)
	`

	args := pgx.NamedArgs{
		"id":         user.ID,
		"email":      user.Email,
		"first_name": user.FirstName,
		"last_name":  user.LastName,
		"phone":      user.Phone,
		"avatar_url": user.AvatarURL,
		"created_at": user.CreatedAt,
		"updated_at": user.UpdatedAt,
	}

	_, err := querier.Exec(ctx, query, args)
	if err != nil {
		if postgres.IsUniqueViolation(err) {
			return domain.AlreadyExistsError
		}
		return fmt.Errorf("insert user: %w", err)
	}

	return nil
}
