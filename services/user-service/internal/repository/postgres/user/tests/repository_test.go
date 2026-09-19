//go:build integration

package repository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/testutils"
	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/domain"
	userRepo "github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/repository/postgres/user"
	"github.com/Krokozabra213/e-commerce_shop/services/user-service/migrations"
	"github.com/google/uuid"
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
func ptr[T any](v T) *T {
	return &v
}

func truncateAll(t *testing.T) {
	t.Helper()
	testutils.TruncateTables(t, testDB.Pool, "user_roles", "users")
}

func newUser() *domain.User {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return &domain.User{
		ID:        uuid.New(),
		Email:     uuid.New().String() + "@test.com",
		FirstName: ptr("John"),
		LastName:  ptr("Doe"),
		Phone:     ptr("+7" + uuid.New().String()[:10]),
		AvatarURL: ptr("https://example.com/avatar.png"),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func createTestUser(t *testing.T, ctx context.Context, repo *userRepo.PostgresUserRepository) *domain.User {
	t.Helper()
	user := newUser()
	require.NoError(t, repo.Create(ctx, user))
	return user
}
