package userRepo

import (
	"context"
	"fmt"

	"github.com/Krokozabra213/e-commerce_shop/infra/postgres"
	tx_manager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *PostgresUserRepository) AddRole(ctx context.Context, userID uuid.UUID, role domain.Role) error {
	tx := tx_manager.ExtractTx(ctx)
	querier := tx_manager.GetQuerier(r.pool, tx)

	query := `
		INSERT INTO user_roles (user_id, role)
		VALUES (@user_id, @role)
	`

	args := pgx.NamedArgs{
		"user_id": userID,
		"role":    string(role),
	}

	_, err := querier.Exec(ctx, query, args)
	if err != nil {
		if postgres.IsUniqueViolation(err) {
			return domain.ErrAlreadyExists
		}
		if postgres.IsForeignKeyViolation(err) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("add role: %w", err)
	}

	return nil
}
