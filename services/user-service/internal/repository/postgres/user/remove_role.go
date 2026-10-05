package userRepo

import (
	"context"
	"fmt"

	txmanager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *PostgresUserRepository) RemoveRole(ctx context.Context, userID uuid.UUID, role domain.Role) error {
	tx := txmanager.ExtractTx(ctx)
	querier := txmanager.GetQuerier(r.pool, tx)

	query := `
		DELETE FROM user_roles
		WHERE user_id = @user_id AND role = @role
	`

	args := pgx.NamedArgs{
		"user_id": userID,
		"role":    string(role),
	}

	tag, err := querier.Exec(ctx, query, args)
	if err != nil {
		return fmt.Errorf("remove role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}
