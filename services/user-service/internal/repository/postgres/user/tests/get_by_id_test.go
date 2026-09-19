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

func TestPostgresUserRepository_GetByID(t *testing.T) {
	ctx := context.Background()
	repo := userRepo.NewPostgresUserRepository(testDB.Pool)

	t.Run("success - returns user without roles", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)

		found, err := repo.GetByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, user.ID, found.ID)
		assert.Equal(t, user.Email, found.Email)
		assert.Empty(t, found.Roles)
	})

	t.Run("success - returns user with single role", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)
		require.NoError(t, repo.AddRole(ctx, user.ID, domain.RoleUser))

		found, err := repo.GetByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, []domain.Role{domain.RoleUser}, found.Roles)
	})

	t.Run("success - returns user with multiple roles", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)
		require.NoError(t, repo.AddRole(ctx, user.ID, domain.RoleUser))
		require.NoError(t, repo.AddRole(ctx, user.ID, domain.RoleManager))
		require.NoError(t, repo.AddRole(ctx, user.ID, domain.RoleAdmin))

		found, err := repo.GetByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Len(t, found.Roles, 3)
		assert.ElementsMatch(t,
			[]domain.Role{domain.RoleUser, domain.RoleManager, domain.RoleAdmin},
			found.Roles,
		)
	})

	t.Run("success - returns user with all nullable fields as nil", func(t *testing.T) {
		defer truncateAll(t)

		now := time.Now().UTC()
		user := &domain.User{
			ID:        uuid.New(),
			Email:     uuid.New().String() + "@test.com",
			CreatedAt: now,
			UpdatedAt: now,
		}
		require.NoError(t, repo.Create(ctx, user))

		found, err := repo.GetByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Nil(t, found.FirstName)
		assert.Nil(t, found.LastName)
		assert.Nil(t, found.Phone)
		assert.Nil(t, found.AvatarURL)
		assert.Nil(t, found.DeletedAt)
	})

	t.Run("error - not found returns ErrNotFound", func(t *testing.T) {
		defer truncateAll(t)

		_, err := repo.GetByID(ctx, uuid.New())
		require.ErrorIs(t, err, domain.NotFoundError)
	})

	t.Run("error - soft-deleted user returns ErrNotFound", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)
		require.NoError(t, repo.SoftDelete(ctx, user.ID))

		_, err := repo.GetByID(ctx, user.ID)
		require.ErrorIs(t, err, domain.NotFoundError)
	})

	t.Run("success - reads within transaction", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		found, err := repo.GetByID(txCtx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, user.ID, found.ID)
	})
}
