package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/Krokozabra213/e-commerce_shop/infra/config"
	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	uniqueViolationCode     = "23505"
	foreignKeyViolationCode = "23503"
)

func NewPostgresClient(cfg *infracfg.PostgresConfig) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
	defer cancel()

	pgxConfig, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("postgres: parse config: %w", err)
	}

	pgxConfig.MaxConns = int32(cfg.MaxConns)
	pgxConfig.MinConns = int32(cfg.MinConns)
	pgxConfig.MaxConnLifetime = cfg.MaxConnLifetime
	pgxConfig.MaxConnIdleTime = cfg.MaxConnIdleTime
	pgxConfig.HealthCheckPeriod = cfg.HealthCheckPeriod

	if pgxConfig.MinConns > pgxConfig.MaxConns {
		return nil, fmt.Errorf("postgres: minConns (%d) > maxConns (%d)",
			pgxConfig.MinConns, pgxConfig.MaxConns)
	}

	var tracerOpts []otelpgx.Option
	tracerOpts = append(tracerOpts, otelpgx.WithTrimSQLInSpanName())
	if cfg.IncludeQueryParams {
		tracerOpts = append(tracerOpts, otelpgx.WithIncludeQueryParameters())
	}
	pgxConfig.ConnConfig.Tracer = otelpgx.NewTracer(tracerOpts...)

	connPool, err := pgxpool.NewWithConfig(ctx, pgxConfig)
	if err != nil {
		return nil, fmt.Errorf("error while creating connection to the database: %w", err)
	}

	if err := connPool.Ping(ctx); err != nil {
		connPool.Close()
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}

	return connPool, nil
}

func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode
}

func IsForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == foreignKeyViolationCode
	}
	return false
}
