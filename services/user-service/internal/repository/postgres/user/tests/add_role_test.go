//go:build integration

package repository_test

import (
	"context"
	"testing"

	tx_manager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/domain"
	userRepo "github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/repository/postgres/user"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresUserRepository_AddRole(t *testing.T) {
	ctx := context.Background()
	repo := userRepo.NewPostgresUserRepository(testDB.Pool)

	t.Run("success - adds single role to user", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)

		err := repo.AddRole(ctx, user.ID, domain.RoleUser)
		require.NoError(t, err)

		roles, err := repo.GetRoles(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, []domain.Role{domain.RoleUser}, roles)
	})

	t.Run("success - adds multiple different roles to same user", func(t *testing.T) {
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

	t.Run("success - same role can be assigned to different users", func(t *testing.T) {
		defer truncateAll(t)

		u1 := createTestUser(t, ctx, repo)
		u2 := createTestUser(t, ctx, repo)

		require.NoError(t, repo.AddRole(ctx, u1.ID, domain.RoleAdmin))
		require.NoError(t, repo.AddRole(ctx, u2.ID, domain.RoleAdmin))

		roles1, _ := repo.GetRoles(ctx, u1.ID)
		roles2, _ := repo.GetRoles(ctx, u2.ID)

		assert.Equal(t, []domain.Role{domain.RoleAdmin}, roles1)
		assert.Equal(t, []domain.Role{domain.RoleAdmin}, roles2)
	})

	t.Run("error - duplicate role returns ErrRoleExists", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)
		require.NoError(t, repo.AddRole(ctx, user.ID, domain.RoleUser))

		err := repo.AddRole(ctx, user.ID, domain.RoleUser)
		require.ErrorIs(t, err, domain.ErrAlreadyExists)
	})

	t.Run("error - non-existent user_id returns ErrNotFound (FK violation)", func(t *testing.T) {
		defer truncateAll(t)

		fakeUserID := uuid.New()
		err := repo.AddRole(ctx, fakeUserID, domain.RoleUser)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("error - invalid role rejected by CHECK constraint", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)

		err := repo.AddRole(ctx, user.ID, domain.Role("ROLE_SUPERADMIN"))
		require.Error(t, err)
		assert.NotErrorIs(t, err, domain.ErrAlreadyExists)
		assert.NotErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("success - adds role within transaction and commits", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		require.NoError(t, repo.AddRole(txCtx, user.ID, domain.RoleManager))
		require.NoError(t, tx.Commit(ctx))

		roles, err := repo.GetRoles(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, []domain.Role{domain.RoleManager}, roles)
	})

	t.Run("success - rollback in transaction cancels role addition", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		require.NoError(t, repo.AddRole(txCtx, user.ID, domain.RoleAdmin))
		require.NoError(t, tx.Rollback(ctx))

		roles, err := repo.GetRoles(ctx, user.ID)
		require.NoError(t, err)
		assert.Empty(t, roles)
	})

	t.Run("success - adding role after removing same role works", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)

		require.NoError(t, repo.AddRole(ctx, user.ID, domain.RoleUser))
		require.NoError(t, repo.RemoveRole(ctx, user.ID, domain.RoleUser))

		err := repo.AddRole(ctx, user.ID, domain.RoleUser)
		require.NoError(t, err)

		roles, _ := repo.GetRoles(ctx, user.ID)
		assert.Equal(t, []domain.Role{domain.RoleUser}, roles)
	})
}
