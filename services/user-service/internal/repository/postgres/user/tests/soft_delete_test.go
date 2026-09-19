//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	tx_manager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/domain"
	userRepo "github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/repository/postgres/user"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresUserRepository_SoftDelete(t *testing.T) {
	ctx := context.Background()
	repo := userRepo.NewPostgresUserRepository(testDB.Pool)

	t.Run("success - marks user as deleted", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)

		err := repo.SoftDelete(ctx, user.ID)
		require.NoError(t, err)

		_, err = repo.GetByID(ctx, user.ID)
		require.ErrorIs(t, err, domain.NotFoundError)

		var deletedAt *time.Time
		err = testDB.Pool.QueryRow(ctx,
			"SELECT deleted_at FROM users WHERE id = $1", user.ID,
		).Scan(&deletedAt)
		require.NoError(t, err)
		require.NotNil(t, deletedAt)
		assert.WithinDuration(t, time.Now().UTC(), *deletedAt, 5*time.Second)
	})

	t.Run("success - updates updated_at along with deleted_at", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)
		originalUpdatedAt := user.UpdatedAt

		time.Sleep(10 * time.Millisecond)

		require.NoError(t, repo.SoftDelete(ctx, user.ID))

		var updatedAt time.Time
		err := testDB.Pool.QueryRow(ctx,
			"SELECT updated_at FROM users WHERE id = $1", user.ID,
		).Scan(&updatedAt)
		require.NoError(t, err)
		assert.True(t, updatedAt.After(originalUpdatedAt),
			"updated_at must be updated on soft delete")
	})

	t.Run("success - roles preserved after soft delete (cascade doesn't fire)", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)
		require.NoError(t, repo.AddRole(ctx, user.ID, domain.RoleUser))
		require.NoError(t, repo.AddRole(ctx, user.ID, domain.RoleAdmin))

		require.NoError(t, repo.SoftDelete(ctx, user.ID))

		var count int
		err := testDB.Pool.QueryRow(ctx,
			"SELECT COUNT(*) FROM user_roles WHERE user_id = $1", user.ID,
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("error - deleting non-existent user returns ErrNotFound", func(t *testing.T) {
		defer truncateAll(t)

		err := repo.SoftDelete(ctx, uuid.New())
		require.ErrorIs(t, err, domain.NotFoundError)
	})

	t.Run("error - deleting already-deleted user returns ErrNotFound", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)

		require.NoError(t, repo.SoftDelete(ctx, user.ID))

		err := repo.SoftDelete(ctx, user.ID)
		require.ErrorIs(t, err, domain.NotFoundError)
	})

	t.Run("success - deletes within transaction and commits", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		err = repo.SoftDelete(txCtx, user.ID)
		require.NoError(t, err)

		require.NoError(t, tx.Commit(ctx))

		_, err = repo.GetByID(ctx, user.ID)
		require.ErrorIs(t, err, domain.NotFoundError)
	})

	t.Run("success - rollback in transaction cancels soft delete", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		require.NoError(t, repo.SoftDelete(txCtx, user.ID))
		require.NoError(t, tx.Rollback(ctx))

		found, err := repo.GetByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, user.ID, found.ID)
	})

	t.Run("success - soft-deleted user excluded from list", func(t *testing.T) {
		defer truncateAll(t)

		u1 := createTestUser(t, ctx, repo)
		u2 := createTestUser(t, ctx, repo)

		require.NoError(t, repo.SoftDelete(ctx, u1.ID))

		users, total, err := repo.List(ctx, domain.ListUsersFilter{Limit: 10, Offset: 0})
		require.NoError(t, err)
		assert.Len(t, users, 1)
		assert.Equal(t, 1, total)
		assert.Equal(t, u2.ID, users[0].ID)
	})
}
