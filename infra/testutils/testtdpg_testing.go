package testutils

import (
	"context"
	"io/fs"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // Register pgx driver for database/sql
)

func SetupTestDatabase(t *testing.T) *TestDatabase {
	t.Helper()

	ctx := context.Background()
	db, err := SetupTestDatabaseShared(ctx)
	if err != nil {
		t.Fatalf("failed to setup test database: %s", err)
	}

	t.Cleanup(func() {
		if err := db.Close(context.Background()); err != nil {
			t.Logf("failed to close test database: %s", err)
		}
	})

	return db
}

func RunMigrations(t *testing.T, connStr string, migrationsFS fs.FS) {
	t.Helper()

	if err := RunMigrationsCtx(context.Background(), connStr, migrationsFS); err != nil {
		t.Fatalf("failed to run migrations: %s", err)
	}
}

func TruncateTables(t *testing.T, pool *pgxpool.Pool, tables ...string) {
	t.Helper()

	if err := TruncateTablesCtx(context.Background(), pool, tables...); err != nil {
		t.Fatalf("failed to truncate tables: %s", err)
	}
}
