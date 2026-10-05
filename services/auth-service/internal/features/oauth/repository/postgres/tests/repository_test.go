//go:build integration

package repository_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/testutils"
	tx_manager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
	userRepo "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/auth/repository/postgres/user"
	oauthrepo "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/oauth/repository/postgres"
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

func TestPostgresOAuthRepository_Create(t *testing.T) {
	ctx := context.Background()
	oauthRepository := oauthrepo.NewPostgresOAuthRepository(testDB.Pool)
	userRepository := userRepo.NewPostgresUserRepository(testDB.Pool)

	t.Run("success - creates oauth account without transaction", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "oauth_accounts", "users")

		user := createTestUser(t, ctx, userRepository)

		account := &domain.OauthAccount{
			ID:             uuid.New(),
			UserID:         user.ID,
			Provider:       domain.OAuthProviderGoogle.String(),
			ProviderUserID: "google-123",
			CreatedAt:      time.Now(),
		}

		err := oauthRepository.Create(ctx, account)
		require.NoError(t, err)

		found, err := oauthRepository.GetByProviderAndProviderUserID(ctx, domain.OAuthProviderGoogle, "google-123")
		require.NoError(t, err)
		assert.Equal(t, account.ID, found.ID)
		assert.Equal(t, account.UserID, found.UserID)
		assert.Equal(t, user.ID, found.UserID)
		assert.Equal(t, account.Provider, found.Provider)
		assert.Equal(t, account.ProviderUserID, found.ProviderUserID)
	})

	t.Run("success - creates oauth account within transaction", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "oauth_accounts", "users")

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		user := &domain.User{
			ID:               uuid.New(),
			Email:            uuid.New().String() + "@test.com",
			PasswordHash:     nil,
			EmailConfirmedAt: nil,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}

		err = userRepository.Create(txCtx, user)
		require.NoError(t, err)

		account := &domain.OauthAccount{
			ID:             uuid.New(),
			UserID:         user.ID,
			Provider:       domain.OAuthProviderGithub.String(),
			ProviderUserID: "github-456",
			CreatedAt:      time.Now(),
		}

		err = oauthRepository.Create(txCtx, account)
		require.NoError(t, err)

		err = tx.Commit(ctx)
		require.NoError(t, err)

		found, err := oauthRepository.GetByProviderAndProviderUserID(ctx, domain.OAuthProviderGithub, "github-456")
		require.NoError(t, err)
		assert.Equal(t, account.ID, found.ID)
		assert.Equal(t, user.ID, found.UserID)
	})

	t.Run("success - creates multiple accounts for same user with different providers", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "oauth_accounts", "users")

		user := createTestUser(t, ctx, userRepository)

		googleAccount := &domain.OauthAccount{
			ID:             uuid.New(),
			UserID:         user.ID,
			Provider:       domain.OAuthProviderGoogle.String(),
			ProviderUserID: "google-999",
			CreatedAt:      time.Now(),
		}

		githubAccount := &domain.OauthAccount{
			ID:             uuid.New(),
			UserID:         user.ID,
			Provider:       domain.OAuthProviderGithub.String(),
			ProviderUserID: "github-999",
			CreatedAt:      time.Now(),
		}

		err := oauthRepository.Create(ctx, googleAccount)
		require.NoError(t, err)

		err = oauthRepository.Create(ctx, githubAccount)
		require.NoError(t, err)

		foundGoogle, err := oauthRepository.GetByProviderAndProviderUserID(ctx, domain.OAuthProviderGoogle, "google-999")
		require.NoError(t, err)
		assert.Equal(t, user.ID, foundGoogle.UserID)

		foundGithub, err := oauthRepository.GetByProviderAndProviderUserID(ctx, domain.OAuthProviderGithub, "github-999")
		require.NoError(t, err)
		assert.Equal(t, user.ID, foundGithub.UserID)
	})

	t.Run("error - duplicate provider and provider_user_id", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "oauth_accounts", "users")

		user1 := createTestUser(t, ctx, userRepository)
		user2 := createTestUser(t, ctx, userRepository)

		account1 := &domain.OauthAccount{
			ID:             uuid.New(),
			UserID:         user1.ID,
			Provider:       domain.OAuthProviderGoogle.String(),
			ProviderUserID: "google-duplicate",
			CreatedAt:      time.Now(),
		}

		err := oauthRepository.Create(ctx, account1)
		require.NoError(t, err)

		account2 := &domain.OauthAccount{
			ID:             uuid.New(),
			UserID:         user2.ID,
			Provider:       domain.OAuthProviderGoogle.String(),
			ProviderUserID: "google-duplicate",
			CreatedAt:      time.Now(),
		}

		err = oauthRepository.Create(ctx, account2)
		require.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrAlreadyExists))
	})

	t.Run("error - duplicate oauth account id", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "oauth_accounts", "users")

		user1 := createTestUser(t, ctx, userRepository)
		user2 := createTestUser(t, ctx, userRepository)

		oauthID := uuid.New()

		account1 := &domain.OauthAccount{
			ID:             oauthID,
			UserID:         user1.ID,
			Provider:       domain.OAuthProviderGoogle.String(),
			ProviderUserID: "google-1",
			CreatedAt:      time.Now(),
		}

		err := oauthRepository.Create(ctx, account1)
		require.NoError(t, err)

		account2 := &domain.OauthAccount{
			ID:             oauthID,
			UserID:         user2.ID,
			Provider:       domain.OAuthProviderGithub.String(),
			ProviderUserID: "github-2",
			CreatedAt:      time.Now(),
		}

		err = oauthRepository.Create(ctx, account2)
		require.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrAlreadyExists))
	})

	t.Run("error - non-existent user_id", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "oauth_accounts", "users")

		account := &domain.OauthAccount{
			ID:             uuid.New(),
			UserID:         uuid.New(),
			Provider:       domain.OAuthProviderGoogle.String(),
			ProviderUserID: "google-123",
			CreatedAt:      time.Now(),
		}

		err := oauthRepository.Create(ctx, account)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "foreign key")
	})

	t.Run("rollback on transaction error", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "oauth_accounts", "users")

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		user := &domain.User{
			ID:               uuid.New(),
			Email:            uuid.New().String() + "@test.com",
			PasswordHash:     nil,
			EmailConfirmedAt: nil,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}

		err = userRepository.Create(txCtx, user)
		require.NoError(t, err)

		account := &domain.OauthAccount{
			ID:             uuid.New(),
			UserID:         user.ID,
			Provider:       domain.OAuthProviderGoogle.String(),
			ProviderUserID: "google-rollback",
			CreatedAt:      time.Now(),
		}

		err = oauthRepository.Create(txCtx, account)
		require.NoError(t, err)

		err = tx.Rollback(ctx)
		require.NoError(t, err)

		found, err := oauthRepository.GetByProviderAndProviderUserID(ctx, domain.OAuthProviderGoogle, "google-rollback")
		assert.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrNotFound))
		assert.Nil(t, found)
	})

	t.Run("allows same provider_user_id for different providers", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "oauth_accounts", "users")

		user1 := createTestUser(t, ctx, userRepository)
		user2 := createTestUser(t, ctx, userRepository)

		providerUserID := "same-id-123"

		googleAccount := &domain.OauthAccount{
			ID:             uuid.New(),
			UserID:         user1.ID,
			Provider:       domain.OAuthProviderGoogle.String(),
			ProviderUserID: providerUserID,
			CreatedAt:      time.Now(),
		}

		githubAccount := &domain.OauthAccount{
			ID:             uuid.New(),
			UserID:         user2.ID,
			Provider:       domain.OAuthProviderGithub.String(),
			ProviderUserID: providerUserID,
			CreatedAt:      time.Now(),
		}

		err := oauthRepository.Create(ctx, googleAccount)
		require.NoError(t, err)

		err = oauthRepository.Create(ctx, githubAccount)
		require.NoError(t, err)
	})

	t.Run("cascade delete when user is deleted", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "oauth_accounts", "users")

		user := createTestUser(t, ctx, userRepository)

		account := &domain.OauthAccount{
			ID:             uuid.New(),
			UserID:         user.ID,
			Provider:       domain.OAuthProviderGoogle.String(),
			ProviderUserID: "google-cascade",
			CreatedAt:      time.Now(),
		}

		err := oauthRepository.Create(ctx, account)
		require.NoError(t, err)

		_, err = testDB.Pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID)
		require.NoError(t, err)

		found, err := oauthRepository.GetByProviderAndProviderUserID(ctx, domain.OAuthProviderGoogle, "google-cascade")
		assert.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrNotFound))
		assert.Nil(t, found)
	})
}

func TestPostgresOAuthRepository_GetByProviderAndProviderUserID(t *testing.T) {
	ctx := context.Background()
	oauthRepository := oauthrepo.NewPostgresOAuthRepository(testDB.Pool)
	userRepository := userRepo.NewPostgresUserRepository(testDB.Pool)

	t.Run("success - finds existing account without transaction", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "oauth_accounts", "users")

		user := createTestUser(t, ctx, userRepository)

		created := &domain.OauthAccount{
			ID:             uuid.New(),
			UserID:         user.ID,
			Provider:       domain.OAuthProviderGoogle.String(),
			ProviderUserID: "google-find-1",
			CreatedAt:      time.Now().UTC().Truncate(time.Microsecond),
		}

		err := oauthRepository.Create(ctx, created)
		require.NoError(t, err)

		found, err := oauthRepository.GetByProviderAndProviderUserID(ctx, domain.OAuthProviderGoogle, "google-find-1")
		require.NoError(t, err)
		require.NotNil(t, found)

		assert.Equal(t, created.ID, found.ID)
		assert.Equal(t, created.UserID, found.UserID)
		assert.Equal(t, user.ID, found.UserID)
		assert.Equal(t, created.Provider, found.Provider)
		assert.Equal(t, created.ProviderUserID, found.ProviderUserID)
		assert.True(t, created.CreatedAt.Equal(found.CreatedAt))
	})

	t.Run("success - finds account within transaction", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "oauth_accounts", "users")

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() {
			tx.Rollback(ctx)
		}()

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		user := &domain.User{
			ID:               uuid.New(),
			Email:            uuid.New().String() + "@test.com",
			PasswordHash:     nil,
			EmailConfirmedAt: nil,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}

		err = userRepository.Create(txCtx, user)
		require.NoError(t, err)

		account := &domain.OauthAccount{
			ID:             uuid.New(),
			UserID:         user.ID,
			Provider:       domain.OAuthProviderGithub.String(),
			ProviderUserID: "github-find-tx",
			CreatedAt:      time.Now(),
		}

		err = oauthRepository.Create(txCtx, account)
		require.NoError(t, err)

		found, err := oauthRepository.GetByProviderAndProviderUserID(txCtx, domain.OAuthProviderGithub, "github-find-tx")
		require.NoError(t, err)
		assert.Equal(t, account.ID, found.ID)
		assert.Equal(t, user.ID, found.UserID)

		err = tx.Commit(ctx)
		require.NoError(t, err)
	})

	t.Run("error - account not found", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "oauth_accounts", "users")

		found, err := oauthRepository.GetByProviderAndProviderUserID(ctx, domain.OAuthProviderGoogle, "nonexistent")
		require.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrNotFound))
		assert.Nil(t, found)
	})

	t.Run("error - wrong provider", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "oauth_accounts", "users")

		user := createTestUser(t, ctx, userRepository)

		account := &domain.OauthAccount{
			ID:             uuid.New(),
			UserID:         user.ID,
			Provider:       domain.OAuthProviderGoogle.String(),
			ProviderUserID: "google-wrong-provider",
			CreatedAt:      time.Now(),
		}

		err := oauthRepository.Create(ctx, account)
		require.NoError(t, err)

		found, err := oauthRepository.GetByProviderAndProviderUserID(ctx, domain.OAuthProviderGithub, "google-wrong-provider")
		require.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrNotFound))
		assert.Nil(t, found)
	})

	t.Run("error - wrong provider_user_id", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "oauth_accounts", "users")

		user := createTestUser(t, ctx, userRepository)

		account := &domain.OauthAccount{
			ID:             uuid.New(),
			UserID:         user.ID,
			Provider:       domain.OAuthProviderGoogle.String(),
			ProviderUserID: "google-correct-id",
			CreatedAt:      time.Now(),
		}

		err := oauthRepository.Create(ctx, account)
		require.NoError(t, err)

		found, err := oauthRepository.GetByProviderAndProviderUserID(ctx, domain.OAuthProviderGoogle, "google-wrong-id")
		require.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrNotFound))
		assert.Nil(t, found)
	})

	t.Run("finds correct account among multiple", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "oauth_accounts", "users")

		user1 := createTestUser(t, ctx, userRepository)
		user2 := createTestUser(t, ctx, userRepository)
		user3 := createTestUser(t, ctx, userRepository)

		accounts := []*domain.OauthAccount{
			{
				ID:             uuid.New(),
				UserID:         user1.ID,
				Provider:       domain.OAuthProviderGoogle.String(),
				ProviderUserID: "google-1",
				CreatedAt:      time.Now(),
			},
			{
				ID:             uuid.New(),
				UserID:         user2.ID,
				Provider:       domain.OAuthProviderGoogle.String(),
				ProviderUserID: "google-2",
				CreatedAt:      time.Now(),
			},
			{
				ID:             uuid.New(),
				UserID:         user3.ID,
				Provider:       domain.OAuthProviderGithub.String(),
				ProviderUserID: "github-1",
				CreatedAt:      time.Now(),
			},
		}

		for _, acc := range accounts {
			err := oauthRepository.Create(ctx, acc)
			require.NoError(t, err)
		}

		found, err := oauthRepository.GetByProviderAndProviderUserID(ctx, domain.OAuthProviderGoogle, "google-2")
		require.NoError(t, err)
		assert.Equal(t, accounts[1].ID, found.ID)
		assert.Equal(t, user2.ID, found.UserID)
	})

	t.Run("does not see uncommitted transaction data", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "oauth_accounts", "users")

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() {
			_ = tx.Rollback(ctx)
		}()

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		user := &domain.User{
			ID:               uuid.New(),
			Email:            uuid.New().String() + "@test.com",
			PasswordHash:     nil,
			EmailConfirmedAt: nil,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}

		err = userRepository.Create(txCtx, user)
		require.NoError(t, err)

		account := &domain.OauthAccount{
			ID:             uuid.New(),
			UserID:         user.ID,
			Provider:       domain.OAuthProviderGoogle.String(),
			ProviderUserID: "google-uncommitted",
			CreatedAt:      time.Now(),
		}

		err = oauthRepository.Create(txCtx, account)
		require.NoError(t, err)

		found, err := oauthRepository.GetByProviderAndProviderUserID(ctx, domain.OAuthProviderGoogle, "google-uncommitted")
		require.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrNotFound))
		assert.Nil(t, found)
	})

	t.Run("finds same provider_user_id for different providers", func(t *testing.T) {
		defer testutils.TruncateTables(t, testDB.Pool, "oauth_accounts", "users")

		user1 := createTestUser(t, ctx, userRepository)
		user2 := createTestUser(t, ctx, userRepository)

		providerUserID := "same-id-456"

		googleAccount := &domain.OauthAccount{
			ID:             uuid.New(),
			UserID:         user1.ID,
			Provider:       domain.OAuthProviderGoogle.String(),
			ProviderUserID: providerUserID,
			CreatedAt:      time.Now(),
		}

		githubAccount := &domain.OauthAccount{
			ID:             uuid.New(),
			UserID:         user2.ID,
			Provider:       domain.OAuthProviderGithub.String(),
			ProviderUserID: providerUserID,
			CreatedAt:      time.Now(),
		}

		err := oauthRepository.Create(ctx, googleAccount)
		require.NoError(t, err)

		err = oauthRepository.Create(ctx, githubAccount)
		require.NoError(t, err)

		foundGoogle, err := oauthRepository.GetByProviderAndProviderUserID(ctx, domain.OAuthProviderGoogle, providerUserID)
		require.NoError(t, err)
		assert.Equal(t, googleAccount.ID, foundGoogle.ID)
		assert.Equal(t, user1.ID, foundGoogle.UserID)
		assert.Equal(t, domain.OAuthProviderGoogle.String(), foundGoogle.Provider)

		foundGithub, err := oauthRepository.GetByProviderAndProviderUserID(ctx, domain.OAuthProviderGithub, providerUserID)
		require.NoError(t, err)
		assert.Equal(t, githubAccount.ID, foundGithub.ID)
		assert.Equal(t, user2.ID, foundGithub.UserID)
		assert.Equal(t, domain.OAuthProviderGithub.String(), foundGithub.Provider)

		assert.NotEqual(t, foundGoogle.ID, foundGithub.ID)
	})
}
