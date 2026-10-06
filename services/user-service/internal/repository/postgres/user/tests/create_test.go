//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	txmanager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/domain"
	userRepo "github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/repository/postgres/user"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresUserRepository_Create(t *testing.T) {
	ctx := context.Background()
	repo := userRepo.NewPostgresUserRepository(testDB.Pool)

	t.Run("success - creates user without transaction", func(t *testing.T) {
		defer truncateAll(t)

		user := newUser()

		err := repo.Create(ctx, user)
		require.NoError(t, err)

		found, err := repo.GetByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, user.ID, found.ID)
		assert.Equal(t, user.Email, found.Email)
		assert.Equal(t, user.FirstName, found.FirstName)
		assert.Equal(t, user.LastName, found.LastName)
		assert.Equal(t, user.Phone, found.Phone)
		assert.Equal(t, user.AvatarURL, found.AvatarURL)
		assert.WithinDuration(t, user.CreatedAt, found.CreatedAt, time.Second)
		assert.WithinDuration(t, user.UpdatedAt, found.UpdatedAt, time.Second)
		assert.Nil(t, found.DeletedAt)
		assert.Empty(t, found.Roles)
	})

	t.Run("success - creates minimal user with only required fields", func(t *testing.T) {
		defer truncateAll(t)

		now := time.Now().UTC()
		user := &domain.User{
			ID:        uuid.New(),
			Email:     uuid.New().String() + "@test.com",
			CreatedAt: now,
			UpdatedAt: now,
		}

		err := repo.Create(ctx, user)
		require.NoError(t, err)

		found, err := repo.GetByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Nil(t, found.FirstName)
		assert.Nil(t, found.LastName)
		assert.Nil(t, found.Phone)
		assert.Nil(t, found.AvatarURL)
	})

	t.Run("success - creates user within transaction and commits", func(t *testing.T) {
		defer truncateAll(t)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := txmanager.CtxWithTx(ctx, tx)

		user := newUser()
		err = repo.Create(txCtx, user)
		require.NoError(t, err)

		require.NoError(t, tx.Commit(ctx))

		found, err := repo.GetByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, user.ID, found.ID)
	})

	t.Run("success - rollback within transaction does not persist user", func(t *testing.T) {
		defer truncateAll(t)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)

		txCtx := txmanager.CtxWithTx(ctx, tx)

		user := newUser()
		require.NoError(t, repo.Create(txCtx, user))

		require.NoError(t, tx.Rollback(ctx))

		_, err = repo.GetByID(ctx, user.ID)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("error - duplicate id returns AlreadyExists", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)

		duplicate := newUser()
		duplicate.ID = user.ID

		err := repo.Create(ctx, duplicate)
		require.ErrorIs(t, err, domain.ErrAlreadyExists)
	})

	t.Run("error - duplicate email returns AlreadyExists", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)

		duplicate := newUser()
		duplicate.Email = user.Email

		err := repo.Create(ctx, duplicate)
		require.ErrorIs(t, err, domain.ErrAlreadyExists)
	})
}
