package userRepo

import (
	"context"
	"errors"
	"fmt"
	"time"

	tx_manager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *PostgresUserRepository) UpdateProfile(
	ctx context.Context,
	id uuid.UUID,
	input domain.UpdateProfileInput,
) (*domain.User, error) {
	tx := tx_manager.ExtractTx(ctx)
	querier := tx_manager.GetQuerier(r.pool, tx)

	query := `
		UPDATE users
		SET 
			first_name = COALESCE(@first_name, first_name),
			last_name  = COALESCE(@last_name, last_name),
			phone      = COALESCE(@phone, phone),
			avatar_url = COALESCE(@avatar_url, avatar_url),
			updated_at = @updated_at
		WHERE id = @id AND deleted_at IS NULL
		RETURNING id, email, first_name, last_name, phone, avatar_url, created_at, updated_at, deleted_at
	`

	args := pgx.NamedArgs{
		"id":         id,
		"first_name": input.FirstName,
		"last_name":  input.LastName,
		"phone":      input.Phone,
		"avatar_url": input.AvatarURL,
		"updated_at": time.Now(),
	}

	row := querier.QueryRow(ctx, query, args)

	var user domain.User
	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.Phone,
		&user.AvatarURL,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.NotFoundError
		}
		return nil, fmt.Errorf("update user profile: %w", err)
	}

	roles, err := r.GetRoles(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("fetch roles after update: %w", err)
	}
	user.Roles = roles

	return &user, nil
}
