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

func TestPostgresUserRepository_RemoveRole(t *testing.T) {
	ctx := context.Background()
	repo := userRepo.NewPostgresUserRepository(testDB.Pool)

	t.Run("success - removes existing role", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)
		require.NoError(t, repo.AddRole(ctx, user.ID, domain.RoleUser))
		require.NoError(t, repo.AddRole(ctx, user.ID, domain.RoleAdmin))

		err := repo.RemoveRole(ctx, user.ID, domain.RoleUser)
		require.NoError(t, err)

		roles, err := repo.GetRoles(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, []domain.Role{domain.RoleAdmin}, roles)
	})

	t.Run("success - removes last role, user has empty roles", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)
		require.NoError(t, repo.AddRole(ctx, user.ID, domain.RoleUser))

		require.NoError(t, repo.RemoveRole(ctx, user.ID, domain.RoleUser))

		roles, err := repo.GetRoles(ctx, user.ID)
		require.NoError(t, err)
		assert.Empty(t, roles)
	})

	t.Run("error - removing non-existent role returns ErrRoleNotFound", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)

		err := repo.RemoveRole(ctx, user.ID, domain.RoleAdmin)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("error - removing role that user never had returns ErrRoleNotFound", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)
		require.NoError(t, repo.AddRole(ctx, user.ID, domain.RoleUser))

		err := repo.RemoveRole(ctx, user.ID, domain.RoleAdmin)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("error - removing role from non-existent user returns ErrRoleNotFound", func(t *testing.T) {
		defer truncateAll(t)

		err := repo.RemoveRole(ctx, uuid.New(), domain.RoleUser)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("success - removing role does not affect other users", func(t *testing.T) {
		defer truncateAll(t)

		u1 := createTestUser(t, ctx, repo)
		u2 := createTestUser(t, ctx, repo)

		require.NoError(t, repo.AddRole(ctx, u1.ID, domain.RoleAdmin))
		require.NoError(t, repo.AddRole(ctx, u2.ID, domain.RoleAdmin))

		require.NoError(t, repo.RemoveRole(ctx, u1.ID, domain.RoleAdmin))

		roles2, _ := repo.GetRoles(ctx, u2.ID)
		assert.Equal(t, []domain.Role{domain.RoleAdmin}, roles2)

		roles1, _ := repo.GetRoles(ctx, u1.ID)
		assert.Empty(t, roles1)
	})

	t.Run("success - removes role within transaction and commits", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)
		require.NoError(t, repo.AddRole(ctx, user.ID, domain.RoleUser))

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := txmanager.CtxWithTx(ctx, tx)

		require.NoError(t, repo.RemoveRole(txCtx, user.ID, domain.RoleUser))
		require.NoError(t, tx.Commit(ctx))

		roles, _ := repo.GetRoles(ctx, user.ID)
		assert.Empty(t, roles)
	})

	t.Run("success - rollback in transaction preserves role", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)
		require.NoError(t, repo.AddRole(ctx, user.ID, domain.RoleManager))

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)

		txCtx := txmanager.CtxWithTx(ctx, tx)

		require.NoError(t, repo.RemoveRole(txCtx, user.ID, domain.RoleManager))
		require.NoError(t, tx.Rollback(ctx))

		roles, _ := repo.GetRoles(ctx, user.ID)
		assert.Equal(t, []domain.Role{domain.RoleManager}, roles)
	})

	t.Run("success - double remove returns ErrRoleNotFound on second call", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)
		require.NoError(t, repo.AddRole(ctx, user.ID, domain.RoleUser))

		require.NoError(t, repo.RemoveRole(ctx, user.ID, domain.RoleUser))

		err := repo.RemoveRole(ctx, user.ID, domain.RoleUser)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})
}
