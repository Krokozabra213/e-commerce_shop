//go:build integration

package refreshtokenRepo_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/testutils"
	txmanager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
	refreshtokenRepo "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/auth/repository/postgres/refresh-token"
	userRepo "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/auth/repository/postgres/user"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/migrations"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testDB *testutils.TestDatabase

func TestMain(m *testing.M) {
	ctx := context.Background()

	db, err := testutils.SetupTestDatabaseShared(ctx)
	if err != nil {
		panic(err)
	}
	testDB = db
	defer func() { _ = testDB.Close(ctx) }()

	if err := testutils.RunMigrationsCtx(ctx, testDB.ConnStr, migrations.Files); err != nil {
		panic(err)
	}

	code := m.Run()
	os.Exit(code)
}

func createTestUser(t *testing.T, ctx context.Context, userRepository *userRepo.PostgresUserRepository) *domain.User {
	t.Helper()

	user := &domain.User{
		ID:               uuid.New(),
		Email:            uuid.New().String() + "@test.com",
		PasswordHash:     nil,
		EmailConfirmedAt: nil,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	err := userRepository.Create(ctx, user)
	require.NoError(t, err)

	return user
}

func createTestRefreshToken(
	t *testing.T,
	ctx context.Context,
	repo *refreshtokenRepo.PostgresRefreshTokenRepository,
	userID uuid.UUID,
) *domain.RefreshToken {
	t.Helper()

	token := &domain.RefreshToken{
		ID:        uuid.New(),
		UserID:    userID,
		TokenHash: uuid.New().String(),
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
		Revoked:   false,
		CreatedAt: time.Now().UTC(),
	}

	err := repo.Create(ctx, token)
	require.NoError(t, err)

	return token
}

func TestPostgresrefreshtokenRepository_Create(t *testing.T) {
	ctx := context.Background()
	repo := refreshtokenRepo.NewPostgresRefreshTokenRepository(testDB.Pool)
	userRepository := userRepo.NewPostgresUserRepository(testDB.Pool)

	t.Run("success - creates refresh token without transaction", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "refresh_tokens", "users")

		user := createTestUser(t, ctx, userRepository)

		token := &domain.RefreshToken{
			ID:        uuid.New(),
			UserID:    user.ID,
			TokenHash: uuid.New().String(),
			ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
			Revoked:   false,
			CreatedAt: time.Now().UTC(),
		}

		err := repo.Create(ctx, token)
		require.NoError(t, err)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() {
			_ = tx.Rollback(ctx)
		}()

		txCtx := txmanager.CtxWithTx(ctx, tx)

		found, err := repo.GetByTokenHashForUpdate(txCtx, token.TokenHash)
		require.NoError(t, err)
		assert.Equal(t, token.ID, found.ID)
		assert.Equal(t, token.UserID, found.UserID)
		assert.Equal(t, token.TokenHash, found.TokenHash)
		assert.False(t, found.Revoked)
	})

	t.Run("success - creates refresh token within transaction", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "refresh_tokens", "users")

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() {
			_ = tx.Rollback(ctx)
		}()

		txCtx := txmanager.CtxWithTx(ctx, tx)

		user := createTestUser(t, txCtx, userRepository)

		token := &domain.RefreshToken{
			ID:        uuid.New(),
			UserID:    user.ID,
			TokenHash: uuid.New().String(),
			ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
			Revoked:   false,
			CreatedAt: time.Now().UTC(),
		}

		err = repo.Create(txCtx, token)
		require.NoError(t, err)

		err = tx.Commit(ctx)
		require.NoError(t, err)

		tx2, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() {
			_ = tx2.Rollback(ctx)
		}()

		txCtx2 := txmanager.CtxWithTx(ctx, tx2)

		found, err := repo.GetByTokenHashForUpdate(txCtx2, token.TokenHash)
		require.NoError(t, err)
		assert.Equal(t, token.ID, found.ID)
		assert.Equal(t, user.ID, found.UserID)
	})

	t.Run("rollback - token not visible after rollback", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "refresh_tokens", "users")

		user := createTestUser(t, ctx, userRepository)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)

		txCtx := txmanager.CtxWithTx(ctx, tx)

		token := &domain.RefreshToken{
			ID:        uuid.New(),
			UserID:    user.ID,
			TokenHash: uuid.New().String(),
			ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
			Revoked:   false,
			CreatedAt: time.Now().UTC(),
		}

		err = repo.Create(txCtx, token)
		require.NoError(t, err)

		err = tx.Rollback(ctx)
		require.NoError(t, err)

		tx2, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() {
			_ = tx2.Rollback(ctx)
		}()
		txCtx2 := txmanager.CtxWithTx(ctx, tx2)

		_, err = repo.GetByTokenHashForUpdate(txCtx2, token.TokenHash)
		assert.True(t, errors.Is(err, domain.ErrNotFound))
	})

	t.Run("error - duplicate token_hash returns AlreadyExistsError", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "refresh_tokens", "users")

		user := createTestUser(t, ctx, userRepository)
		duplicateHash := uuid.New().String()

		token1 := &domain.RefreshToken{
			ID:        uuid.New(),
			UserID:    user.ID,
			TokenHash: duplicateHash,
			ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
			Revoked:   false,
			CreatedAt: time.Now().UTC(),
		}
		err := repo.Create(ctx, token1)
		require.NoError(t, err)

		token2 := &domain.RefreshToken{
			ID:        uuid.New(),
			UserID:    user.ID,
			TokenHash: duplicateHash, // тот же хеш
			ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
			Revoked:   false,
			CreatedAt: time.Now().UTC(),
		}
		err = repo.Create(ctx, token2)
		require.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrAlreadyExists))
	})

	t.Run("error - foreign key violation for non-existent user_id", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "refresh_tokens", "users")

		token := &domain.RefreshToken{
			ID:        uuid.New(),
			UserID:    uuid.New(),
			TokenHash: uuid.New().String(),
			ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
			Revoked:   false,
			CreatedAt: time.Now().UTC(),
		}

		err := repo.Create(ctx, token)
		require.Error(t, err)
		assert.False(t, errors.Is(err, domain.ErrAlreadyExists))
	})
}

func TestPostgresrefreshtokenRepository_GetByTokenHashForUpdate(t *testing.T) {
	ctx := context.Background()
	repo := refreshtokenRepo.NewPostgresRefreshTokenRepository(testDB.Pool)
	userRepository := userRepo.NewPostgresUserRepository(testDB.Pool)

	t.Run("success - returns token by hash within transaction", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "refresh_tokens", "users")

		user := createTestUser(t, ctx, userRepository)
		token := createTestRefreshToken(t, ctx, repo, user.ID)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() {
			_ = tx.Rollback(ctx)
		}()

		txCtx := txmanager.CtxWithTx(ctx, tx)

		found, err := repo.GetByTokenHashForUpdate(txCtx, token.TokenHash)
		require.NoError(t, err)
		assert.Equal(t, token.ID, found.ID)
		assert.Equal(t, token.UserID, found.UserID)
		assert.Equal(t, token.TokenHash, found.TokenHash)
		assert.False(t, found.Revoked)
		assert.WithinDuration(t, token.ExpiresAt, found.ExpiresAt, time.Second)
	})

	t.Run("success - FOR UPDATE locks row (concurrent read blocks)", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "refresh_tokens", "users")

		user := createTestUser(t, ctx, userRepository)
		token := createTestRefreshToken(t, ctx, repo, user.ID)

		tx1, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)

		defer func() {
			_ = tx1.Rollback(ctx)
		}()
		txCtx1 := txmanager.CtxWithTx(ctx, tx1)

		found1, err := repo.GetByTokenHashForUpdate(txCtx1, token.TokenHash)
		require.NoError(t, err)
		assert.Equal(t, token.ID, found1.ID)

		tx2, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)

		defer func() {
			_ = tx2.Rollback(ctx)
		}()
		txCtx2 := txmanager.CtxWithTx(ctx, tx2)

		timeoutCtx, cancel := context.WithTimeout(txCtx2, 500*time.Millisecond)
		defer cancel()

		_, err = repo.GetByTokenHashForUpdate(timeoutCtx, token.TokenHash)
		assert.Error(t, err)

		err = tx1.Rollback(ctx)
		require.NoError(t, err)
	})

	t.Run("error - returns NotFoundError for non-existent hash", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "refresh_tokens", "users")

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() {
			_ = tx.Rollback(ctx)
		}()

		txCtx := txmanager.CtxWithTx(ctx, tx)

		_, err = repo.GetByTokenHashForUpdate(txCtx, "non-existent-hash")
		require.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrNotFound))
	})
}

func TestPostgresrefreshtokenRepository_Revoke(t *testing.T) {
	ctx := context.Background()
	repo := refreshtokenRepo.NewPostgresRefreshTokenRepository(testDB.Pool)
	userRepository := userRepo.NewPostgresUserRepository(testDB.Pool)

	t.Run("success - revokes token", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "refresh_tokens", "users")

		user := createTestUser(t, ctx, userRepository)
		token := createTestRefreshToken(t, ctx, repo, user.ID)

		err := repo.Revoke(ctx, token.TokenHash)
		require.NoError(t, err)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() {
			_ = tx.Rollback(ctx)
		}()

		txCtx := txmanager.CtxWithTx(ctx, tx)

		found, err := repo.GetByTokenHashForUpdate(txCtx, token.TokenHash)
		require.NoError(t, err)
		assert.True(t, found.Revoked)
	})

	t.Run("success - revokes token within transaction", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "refresh_tokens", "users")

		user := createTestUser(t, ctx, userRepository)
		token := createTestRefreshToken(t, ctx, repo, user.ID)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() {
			_ = tx.Rollback(ctx)
		}()

		txCtx := txmanager.CtxWithTx(ctx, tx)

		err = repo.Revoke(txCtx, token.TokenHash)
		require.NoError(t, err)

		err = tx.Commit(ctx)
		require.NoError(t, err)

		tx2, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)

		defer func() {
			_ = tx2.Rollback(ctx)
		}()
		txCtx2 := txmanager.CtxWithTx(ctx, tx2)

		found, err := repo.GetByTokenHashForUpdate(txCtx2, token.TokenHash)
		require.NoError(t, err)
		assert.True(t, found.Revoked)
	})

	t.Run("rollback - token not revoked after rollback", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "refresh_tokens", "users")

		user := createTestUser(t, ctx, userRepository)
		token := createTestRefreshToken(t, ctx, repo, user.ID)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)

		txCtx := txmanager.CtxWithTx(ctx, tx)

		err = repo.Revoke(txCtx, token.TokenHash)
		require.NoError(t, err)

		err = tx.Rollback(ctx)
		require.NoError(t, err)

		tx2, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)

		defer func() {
			_ = tx2.Rollback(ctx)
		}()
		txCtx2 := txmanager.CtxWithTx(ctx, tx2)

		found, err := repo.GetByTokenHashForUpdate(txCtx2, token.TokenHash)
		require.NoError(t, err)
		assert.False(t, found.Revoked)
	})

	t.Run("error - returns NotFoundError for non-existent hash", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "refresh_tokens", "users")

		err := repo.Revoke(ctx, "non-existent-hash")
		require.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrNotFound))
	})

	t.Run("idempotency - revoking already revoked token does not error", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "refresh_tokens", "users")

		user := createTestUser(t, ctx, userRepository)
		token := createTestRefreshToken(t, ctx, repo, user.ID)

		err := repo.Revoke(ctx, token.TokenHash)
		require.NoError(t, err)

		err = repo.Revoke(ctx, token.TokenHash)
		require.NoError(t, err)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() {
			_ = tx.Rollback(ctx)
		}()

		txCtx := txmanager.CtxWithTx(ctx, tx)

		found, err := repo.GetByTokenHashForUpdate(txCtx, token.TokenHash)
		require.NoError(t, err)
		assert.True(t, found.Revoked)
	})
}
