package emailverificationRepo

import "github.com/jackc/pgx/v5/pgxpool"

type PostgresEmailVerificationRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresEmailVerificationRepository(pool *pgxpool.Pool) *PostgresEmailVerificationRepository {
	return &PostgresEmailVerificationRepository{pool: pool}
}
