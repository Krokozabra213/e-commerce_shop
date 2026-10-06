//go:build integration

package repository_test

import (
	"context"
	"testing"

	txmanager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/domain"
	userRepo "github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/repository/postgres/user"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresUserRepository_GetRoles(t *testing.T) {
	ctx := context.Background()
	repo := userRepo.NewPostgresUserRepository(testDB.Pool)

	t.Run("success - returns empty slice for user without roles", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)

		roles, err := repo.GetRoles(ctx, user.ID)
		require.NoError(t, err)
		assert.Empty(t, roles)
	})

	t.Run("success - returns single role", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)
		require.NoError(t, repo.AddRole(ctx, user.ID, domain.RoleUser))

		roles, err := repo.GetRoles(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, []domain.Role{domain.RoleUser}, roles)
	})

	t.Run("success - returns all roles for user with multiple roles", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)
		require.NoError(t, repo.AddRole(ctx, user.ID, domain.RoleUser))
		require.NoError(t, repo.AddRole(ctx, user.ID, domain.RoleManager))
		require.NoError(t, repo.AddRole(ctx, user.ID, domain.RoleAdmin))

		roles, err := repo.GetRoles(ctx, user.ID)
		require.NoError(t, err)
		assert.Len(t, roles, 3)
		assert.ElementsMatch(t,
			[]domain.Role{domain.RoleUser, domain.RoleManager, domain.RoleAdmin},
			roles,
		)
	})

	t.Run("success - returns empty for non-existent user (no FK error)", func(t *testing.T) {
		defer truncateAll(t)

		roles, err := repo.GetRoles(ctx, uuid.New())
		require.NoError(t, err)
		assert.Empty(t, roles)
	})

	t.Run("success - roles reflect additions and removals", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)

		require.NoError(t, repo.AddRole(ctx, user.ID, domain.RoleUser))
		require.NoError(t, repo.AddRole(ctx, user.ID, domain.RoleManager))
		require.NoError(t, repo.AddRole(ctx, user.ID, domain.RoleAdmin))

		require.NoError(t, repo.RemoveRole(ctx, user.ID, domain.RoleManager))

		roles, err := repo.GetRoles(ctx, user.ID)
		require.NoError(t, err)
		assert.Len(t, roles, 2)
		assert.ElementsMatch(t,
			[]domain.Role{domain.RoleUser, domain.RoleAdmin},
			roles,
		)
	})

	t.Run("success - reads roles within transaction", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)
		require.NoError(t, repo.AddRole(ctx, user.ID, domain.RoleAdmin))

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := txmanager.CtxWithTx(ctx, tx)

		roles, err := repo.GetRoles(txCtx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, []domain.Role{domain.RoleAdmin}, roles)
	})

	t.Run("success - sees uncommitted role within same transaction", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := txmanager.CtxWithTx(ctx, tx)

		require.NoError(t, repo.AddRole(txCtx, user.ID, domain.RoleManager))

		roles, err := repo.GetRoles(txCtx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, []domain.Role{domain.RoleManager}, roles)

		rolesOutside, err := repo.GetRoles(ctx, user.ID)
		require.NoError(t, err)
		assert.Empty(t, rolesOutside)
	})
}
