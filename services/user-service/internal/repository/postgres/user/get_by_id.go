package userRepo

import (
	"context"
	"errors"
	"fmt"

	txmanager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *PostgresUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	tx := txmanager.ExtractTx(ctx)
	querier := txmanager.GetQuerier(r.pool, tx)

	query := `
		SELECT 
			u.id, u.email, u.first_name, u.last_name, u.phone, u.avatar_url,
			u.created_at, u.updated_at, u.deleted_at,
			COALESCE(ARRAY_AGG(ur.role) FILTER (WHERE ur.role IS NOT NULL), '{}') AS roles
		FROM users u
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		WHERE u.id = @id AND u.deleted_at IS NULL
		GROUP BY u.id
	`

	args := pgx.NamedArgs{"id": id}

	row := querier.QueryRow(ctx, query, args)

	user, err := scanUserWithRoles(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("scan user by id: %w", err)
	}

	return user, nil
}
