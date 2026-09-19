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

func TestPostgresUserRepository_UpdateProfile(t *testing.T) {
	ctx := context.Background()
	repo := userRepo.NewPostgresUserRepository(testDB.Pool)

	t.Run("success - updates all fields at once", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)

		input := domain.UpdateProfileInput{
			FirstName: ptr("Jane"),
			LastName:  ptr("Smith"),
			Phone:     ptr("+79991234567"),
			AvatarURL: ptr("https://new.example.com/avatar.png"),
		}

		updated, err := repo.UpdateProfile(ctx, user.ID, input)
		require.NoError(t, err)

		assert.Equal(t, user.ID, updated.ID)
		assert.Equal(t, "Jane", *updated.FirstName)
		assert.Equal(t, "Smith", *updated.LastName)
		assert.Equal(t, "+79991234567", *updated.Phone)
		assert.Equal(t, "https://new.example.com/avatar.png", *updated.AvatarURL)

		assert.True(t, updated.UpdatedAt.After(user.UpdatedAt) || updated.UpdatedAt.Equal(user.UpdatedAt),
			"updated_at should be >= original")
	})

	t.Run("success - partial update: only first_name changes, others preserved", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)
		originalLastName := *user.LastName
		originalPhone := *user.Phone

		input := domain.UpdateProfileInput{
			FirstName: ptr("UpdatedName"),
		}

		updated, err := repo.UpdateProfile(ctx, user.ID, input)
		require.NoError(t, err)

		assert.Equal(t, "UpdatedName", *updated.FirstName)
		assert.Equal(t, originalLastName, *updated.LastName)
		assert.Equal(t, originalPhone, *updated.Phone)
	})

	t.Run("success - empty input does not change any fields (except updated_at)", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)

		input := domain.UpdateProfileInput{}

		updated, err := repo.UpdateProfile(ctx, user.ID, input)
		require.NoError(t, err)

		assert.Equal(t, *user.FirstName, *updated.FirstName)
		assert.Equal(t, *user.LastName, *updated.LastName)
		assert.Equal(t, *user.Phone, *updated.Phone)
		assert.Equal(t, *user.AvatarURL, *updated.AvatarURL)
	})

	t.Run("success - returns user with roles populated", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)
		require.NoError(t, repo.AddRole(ctx, user.ID, domain.RoleAdmin))
		require.NoError(t, repo.AddRole(ctx, user.ID, domain.RoleUser))

		input := domain.UpdateProfileInput{FirstName: ptr("Bob")}
		updated, err := repo.UpdateProfile(ctx, user.ID, input)
		require.NoError(t, err)

		assert.ElementsMatch(t,
			[]domain.Role{domain.RoleAdmin, domain.RoleUser},
			updated.Roles,
		)
	})

	t.Run("error - user not found returns ErrNotFound", func(t *testing.T) {
		defer truncateAll(t)

		input := domain.UpdateProfileInput{FirstName: ptr("Ghost")}
		_, err := repo.UpdateProfile(ctx, uuid.New(), input)
		require.ErrorIs(t, err, domain.NotFoundError)
	})

	t.Run("error - updating soft-deleted user returns ErrNotFound", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)
		require.NoError(t, repo.SoftDelete(ctx, user.ID))

		input := domain.UpdateProfileInput{FirstName: ptr("Zombie")}
		_, err := repo.UpdateProfile(ctx, user.ID, input)
		require.ErrorIs(t, err, domain.NotFoundError)
	})

	t.Run("success - updates within transaction and commits", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		input := domain.UpdateProfileInput{FirstName: ptr("TxUpdate")}
		_, err = repo.UpdateProfile(txCtx, user.ID, input)
		require.NoError(t, err)

		require.NoError(t, tx.Commit(ctx))

		found, err := repo.GetByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, "TxUpdate", *found.FirstName)
	})

	t.Run("success - rollback in transaction reverts update", func(t *testing.T) {
		defer truncateAll(t)

		user := createTestUser(t, ctx, repo)
		originalName := *user.FirstName

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		input := domain.UpdateProfileInput{FirstName: ptr("WillBeReverted")}
		_, err = repo.UpdateProfile(txCtx, user.ID, input)
		require.NoError(t, err)

		require.NoError(t, tx.Rollback(ctx))

		found, err := repo.GetByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, originalName, *found.FirstName)
	})
}
