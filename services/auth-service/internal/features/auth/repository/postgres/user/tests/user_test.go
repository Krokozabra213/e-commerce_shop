//go:build integration

package repository_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/testutils"
	txmanager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
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

func createTestUserWithEmail(t *testing.T, ctx context.Context, userRepository *userRepo.PostgresUserRepository, email string) *domain.User {
	t.Helper()

	user := &domain.User{
		ID:               uuid.New(),
		Email:            email,
		PasswordHash:     nil,
		EmailConfirmedAt: nil,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	err := userRepository.Create(ctx, user)
	require.NoError(t, err)

	return user
}

func TestPostgresUserRepository_Create(t *testing.T) {
	ctx := context.Background()
	repo := userRepo.NewPostgresUserRepository(testDB.Pool)
	pass := "hashed_password"

	t.Run("success - creates user without transaction", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "users")

		user := &domain.User{
			ID:               uuid.New(),
			Email:            uuid.New().String() + "@test.com",
			PasswordHash:     &pass,
			EmailConfirmedAt: nil,
			CreatedAt:        time.Now().UTC(),
			UpdatedAt:        time.Now().UTC(),
		}

		err := repo.Create(ctx, user)
		require.NoError(t, err)

		found, err := repo.GetByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, user.ID, found.ID)
		assert.Equal(t, user.Email, found.Email)
		assert.Equal(t, user.PasswordHash, found.PasswordHash)
		assert.Nil(t, found.EmailConfirmedAt)
	})

	t.Run("success - creates user within transaction", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "users")

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() {
			_ = tx.Rollback(ctx)
		}()

		txCtx := txmanager.CtxWithTx(ctx, tx)

		user := &domain.User{
			ID:               uuid.New(),
			Email:            uuid.New().String() + "@test.com",
			PasswordHash:     &pass,
			EmailConfirmedAt: nil,
			CreatedAt:        time.Now().UTC(),
			UpdatedAt:        time.Now().UTC(),
		}

		err = repo.Create(txCtx, user)
		require.NoError(t, err)

		err = tx.Commit(ctx)
		require.NoError(t, err)

		found, err := repo.GetByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, user.ID, found.ID)
		assert.Equal(t, user.Email, found.Email)
	})

	t.Run("rollback - user not visible after rollback", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "users")

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)

		txCtx := txmanager.CtxWithTx(ctx, tx)

		user := &domain.User{
			ID:               uuid.New(),
			Email:            uuid.New().String() + "@test.com",
			PasswordHash:     &pass,
			EmailConfirmedAt: nil,
			CreatedAt:        time.Now().UTC(),
			UpdatedAt:        time.Now().UTC(),
		}

		err = repo.Create(txCtx, user)
		require.NoError(t, err)

		err = tx.Rollback(ctx)
		require.NoError(t, err)

		_, err = repo.GetByID(ctx, user.ID)
		assert.True(t, errors.Is(err, domain.ErrNotFound))
	})

	t.Run("error - duplicate email returns AlreadyExistsError", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "users")

		email := uuid.New().String() + "@duplicate.com"

		user1 := &domain.User{
			ID:           uuid.New(),
			Email:        email,
			PasswordHash: &pass,
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}
		err := repo.Create(ctx, user1)
		require.NoError(t, err)

		user2 := &domain.User{
			ID:           uuid.New(),
			Email:        email,
			PasswordHash: &pass,
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}
		err = repo.Create(ctx, user2)
		require.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrAlreadyExists))
	})
}

func TestPostgresUserRepository_GetByEmail(t *testing.T) {
	ctx := context.Background()
	repo := userRepo.NewPostgresUserRepository(testDB.Pool)
	pass := "hashed_password"

	t.Run("success - returns user by email", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "users")

		email := uuid.New().String() + "@getbyemail.com"
		user := createTestUserWithEmail(t, ctx, repo, email)

		found, err := repo.GetByEmail(ctx, email)
		require.NoError(t, err)
		assert.Equal(t, user.ID, found.ID)
		assert.Equal(t, user.Email, found.Email)
	})

	t.Run("success - returns user by email within transaction", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "users")

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() {
			_ = tx.Rollback(ctx)
		}()

		txCtx := txmanager.CtxWithTx(ctx, tx)

		email := uuid.New().String() + "@txemail.com"
		user := &domain.User{
			ID:           uuid.New(),
			Email:        email,
			PasswordHash: &pass,
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}
		err = repo.Create(txCtx, user)
		require.NoError(t, err)

		found, err := repo.GetByEmail(txCtx, email)
		require.NoError(t, err)
		assert.Equal(t, user.ID, found.ID)
		assert.Equal(t, user.Email, found.Email)
	})

	t.Run("error - returns NotFoundError for non-existent email", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "users")

		_, err := repo.GetByEmail(ctx, "nonexistent@example.com")
		require.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrNotFound))
	})

	t.Run("error - email is case-sensitive (if DB collation requires it)", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "users")

		email := "CaseSensitive@test.com"
		createTestUserWithEmail(t, ctx, repo, email)

		found, err := repo.GetByEmail(ctx, "casesensitive@test.com")
		if err != nil {
			assert.True(t, errors.Is(err, domain.ErrNotFound))
		} else {
			assert.Equal(t, email, found.Email)
		}
	})
}

func TestPostgresUserRepository_GetByID(t *testing.T) {
	ctx := context.Background()
	repo := userRepo.NewPostgresUserRepository(testDB.Pool)
	pass := "hashed_password"

	t.Run("success - returns user by id", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "users")

		user := createTestUser(t, ctx, repo)

		found, err := repo.GetByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, user.ID, found.ID)
		assert.Equal(t, user.Email, found.Email)
	})

	t.Run("success - returns user by id within transaction", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "users")

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() {
			_ = tx.Rollback(ctx)
		}()

		txCtx := txmanager.CtxWithTx(ctx, tx)

		user := &domain.User{
			ID:           uuid.New(),
			Email:        uuid.New().String() + "@txid.com",
			PasswordHash: &pass,
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}
		err = repo.Create(txCtx, user)
		require.NoError(t, err)

		found, err := repo.GetByID(txCtx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, user.ID, found.ID)
	})

	t.Run("error - returns NotFoundError for non-existent id", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "users")

		_, err := repo.GetByID(ctx, uuid.New())
		require.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrNotFound))
	})
}

func TestPostgresUserRepository_ConfirmEmail(t *testing.T) {
	ctx := context.Background()
	repo := userRepo.NewPostgresUserRepository(testDB.Pool)

	t.Run("success - confirms email and sets email_confirmed_at", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "users")

		user := createTestUser(t, ctx, repo)
		require.Nil(t, user.EmailConfirmedAt)

		err := repo.ConfirmEmail(ctx, user.ID)
		require.NoError(t, err)

		found, err := repo.GetByID(ctx, user.ID)
		require.NoError(t, err)
		require.NotNil(t, found.EmailConfirmedAt)
		assert.WithinDuration(t, time.Now().UTC(), *found.EmailConfirmedAt, 5*time.Second)
	})

	t.Run("success - confirms email within transaction", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "users")

		user := createTestUser(t, ctx, repo)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() {
			_ = tx.Rollback(ctx)
		}()

		txCtx := txmanager.CtxWithTx(ctx, tx)

		err = repo.ConfirmEmail(txCtx, user.ID)
		require.NoError(t, err)

		err = tx.Commit(ctx)
		require.NoError(t, err)

		found, err := repo.GetByID(ctx, user.ID)
		require.NoError(t, err)
		require.NotNil(t, found.EmailConfirmedAt)
	})

	t.Run("rollback - email not confirmed after rollback", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "users")

		user := createTestUser(t, ctx, repo)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)

		txCtx := txmanager.CtxWithTx(ctx, tx)

		err = repo.ConfirmEmail(txCtx, user.ID)
		require.NoError(t, err)

		err = tx.Rollback(ctx)
		require.NoError(t, err)

		found, err := repo.GetByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Nil(t, found.EmailConfirmedAt)
	})

	t.Run("error - returns NotFoundError for non-existent user", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "users")

		err := repo.ConfirmEmail(ctx, uuid.New())
		require.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrNotFound))
	})

	t.Run("idempotency - calling confirm twice does not error", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "users")

		user := createTestUser(t, ctx, repo)

		err := repo.ConfirmEmail(ctx, user.ID)
		require.NoError(t, err)

		err = repo.ConfirmEmail(ctx, user.ID)
		require.NoError(t, err)

		found, err := repo.GetByID(ctx, user.ID)
		require.NoError(t, err)
		require.NotNil(t, found.EmailConfirmedAt)
	})
}
