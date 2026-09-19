package testutils

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

type TestDatabase struct {
	Container *postgres.PostgresContainer
	Pool      *pgxpool.Pool
	ConnStr   string
}

func (td *TestDatabase) Close(ctx context.Context) error {
	if td.Pool != nil {
		td.Pool.Close()
	}
	if td.Container != nil {
		return td.Container.Terminate(ctx)
	}
	return nil
}

func SetupTestDatabaseShared(ctx context.Context) (*TestDatabase, error) {
	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start postgres container: %w", err)
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = pgContainer.Terminate(ctx)
		return nil, fmt.Errorf("failed to get connection string: %w", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		_ = pgContainer.Terminate(ctx)
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		_ = pgContainer.Terminate(ctx)
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &TestDatabase{
		Container: pgContainer,
		Pool:      pool,
		ConnStr:   connStr,
	}, nil
}

func RunMigrationsCtx(ctx context.Context, connStr string, migrationsFS fs.FS) error {
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return fmt.Errorf("failed to open DB for migrations: %w", err)
	}
	defer func() { _ = db.Close() }()

	if err := goose.Up(db, "."); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	return nil
}

func TruncateTablesCtx(ctx context.Context, pool *pgxpool.Pool, tables ...string) error {
	for _, table := range tables {
		_, err := pool.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", table))
		if err != nil {
			return fmt.Errorf("failed to truncate table %s: %w", table, err)
		}
	}
	return nil
}

func ResetMigrationsCtx(ctx context.Context, connStr string, migrationsFS fs.FS) error {
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return fmt.Errorf("failed to open DB for migrations: %w", err)
	}
	defer func() { _ = db.Close() }()

	if err := goose.Reset(db, "."); err != nil {
		return fmt.Errorf("failed to reset migrations: %w", err)
	}
	return nil
}
