package userRepo

import (
	"context"
	"fmt"

	txmanager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *PostgresUserRepository) List(ctx context.Context, filter domain.ListUsersFilter) ([]*domain.User, int, error) {
	tx := txmanager.ExtractTx(ctx)
	querier := txmanager.GetQuerier(r.pool, tx)

	var total int
	countQuery := `SELECT COUNT(*) FROM users WHERE deleted_at IS NULL`
	if err := querier.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	if total == 0 {
		return []*domain.User{}, 0, nil
	}

	query := `
		SELECT 
			u.id, u.email, u.first_name, u.last_name, u.phone, u.avatar_url,
			u.created_at, u.updated_at, u.deleted_at,
			COALESCE(ARRAY_AGG(ur.role) FILTER (WHERE ur.role IS NOT NULL), '{}') AS roles
		FROM users u
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		WHERE u.deleted_at IS NULL
		GROUP BY u.id
		ORDER BY u.created_at DESC
		LIMIT @limit OFFSET @offset
	`

	args := pgx.NamedArgs{
		"limit":  filter.Limit,
		"offset": filter.Offset,
	}

	rows, err := querier.Query(ctx, query, args)
	if err != nil {
		return nil, 0, fmt.Errorf("query users list: %w", err)
	}
	defer rows.Close()

	users := make([]*domain.User, 0, filter.Limit)
	for rows.Next() {
		user, err := scanUserWithRoles(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan user in list: %w", err)
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate users: %w", err)
	}

	return users, total, nil
}
