package userRepo

import (
	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresUserRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresUserRepository(pool *pgxpool.Pool) *PostgresUserRepository {
	return &PostgresUserRepository{pool: pool}
}

func scanUserWithRoles(row pgx.Row) (*domain.User, error) {
	var user domain.User
	var roles []string

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
		&roles,
	)
	if err != nil {
		return nil, err
	}

	user.Roles = make([]domain.Role, len(roles))
	for i, r := range roles {
		user.Roles[i] = domain.Role(r)
	}

	return &user, nil
}
