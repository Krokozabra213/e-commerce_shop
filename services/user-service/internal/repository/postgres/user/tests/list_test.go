//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/domain"
	userRepo "github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/repository/postgres/user"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresUserRepository_List(t *testing.T) {
	ctx := context.Background()
	repo := userRepo.NewPostgresUserRepository(testDB.Pool)

	t.Run("success - empty database returns empty slice and zero total", func(t *testing.T) {
		defer truncateAll(t)

		users, total, err := repo.List(ctx, domain.ListUsersFilter{Limit: 10, Offset: 0})
		require.NoError(t, err)
		assert.Empty(t, users)
		assert.Equal(t, 0, total)
	})

	t.Run("success - returns all users when count is less than limit", func(t *testing.T) {
		defer truncateAll(t)

		u1 := createTestUser(t, ctx, repo)
		u2 := createTestUser(t, ctx, repo)
		u3 := createTestUser(t, ctx, repo)

		users, total, err := repo.List(ctx, domain.ListUsersFilter{Limit: 10, Offset: 0})
		require.NoError(t, err)
		assert.Len(t, users, 3)
		assert.Equal(t, 3, total)

		ids := make([]uuid.UUID, len(users))
		for i, u := range users {
			ids[i] = u.ID
		}
		assert.ElementsMatch(t, []uuid.UUID{u1.ID, u2.ID, u3.ID}, ids)
	})

	t.Run("success - returns users sorted by created_at DESC (newest first)", func(t *testing.T) {
		defer truncateAll(t)

		now := time.Now().UTC()
		older := &domain.User{
			ID:        uuid.New(),
			Email:     uuid.New().String() + "@test.com",
			CreatedAt: now.Add(-2 * time.Hour),
			UpdatedAt: now.Add(-2 * time.Hour),
		}
		middle := &domain.User{
			ID:        uuid.New(),
			Email:     uuid.New().String() + "@test.com",
			CreatedAt: now.Add(-1 * time.Hour),
			UpdatedAt: now.Add(-1 * time.Hour),
		}
		newest := &domain.User{
			ID:        uuid.New(),
			Email:     uuid.New().String() + "@test.com",
			CreatedAt: now,
			UpdatedAt: now,
		}

		require.NoError(t, repo.Create(ctx, older))
		require.NoError(t, repo.Create(ctx, middle))
		require.NoError(t, repo.Create(ctx, newest))

		users, total, err := repo.List(ctx, domain.ListUsersFilter{Limit: 10, Offset: 0})
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		require.Len(t, users, 3)

		assert.Equal(t, newest.ID, users[0].ID)
		assert.Equal(t, middle.ID, users[1].ID)
		assert.Equal(t, older.ID, users[2].ID)
	})

	t.Run("success - pagination works correctly with limit and offset", func(t *testing.T) {
		defer truncateAll(t)

		for i := 0; i < 5; i++ {
			createTestUser(t, ctx, repo)
		}

		page1, total, err := repo.List(ctx, domain.ListUsersFilter{Limit: 2, Offset: 0})
		require.NoError(t, err)
		assert.Len(t, page1, 2)
		assert.Equal(t, 5, total)

		page2, total, err := repo.List(ctx, domain.ListUsersFilter{Limit: 2, Offset: 2})
		require.NoError(t, err)
		assert.Len(t, page2, 2)
		assert.Equal(t, 5, total)

		page3, total, err := repo.List(ctx, domain.ListUsersFilter{Limit: 2, Offset: 4})
		require.NoError(t, err)
		assert.Len(t, page3, 1)
		assert.Equal(t, 5, total)

		allIDs := make(map[uuid.UUID]struct{})
		for _, u := range page1 {
			allIDs[u.ID] = struct{}{}
		}
		for _, u := range page2 {
			_, exists := allIDs[u.ID]
			assert.False(t, exists, "page2 must not contain users from page1")
			allIDs[u.ID] = struct{}{}
		}
		for _, u := range page3 {
			_, exists := allIDs[u.ID]
			assert.False(t, exists, "page3 must not contain duplicates")
		}
	})

	t.Run("success - offset beyond total returns empty slice but correct total", func(t *testing.T) {
		defer truncateAll(t)

		createTestUser(t, ctx, repo)
		createTestUser(t, ctx, repo)

		users, total, err := repo.List(ctx, domain.ListUsersFilter{Limit: 10, Offset: 100})
		require.NoError(t, err)
		assert.Empty(t, users)
		assert.Equal(t, 2, total) // total считается независимо от offset
	})

	t.Run("success - soft-deleted users excluded from list and total", func(t *testing.T) {
		defer truncateAll(t)

		active1 := createTestUser(t, ctx, repo)
		deleted := createTestUser(t, ctx, repo)
		active2 := createTestUser(t, ctx, repo)

		require.NoError(t, repo.SoftDelete(ctx, deleted.ID))

		users, total, err := repo.List(ctx, domain.ListUsersFilter{Limit: 10, Offset: 0})
		require.NoError(t, err)
		assert.Len(t, users, 2)
		assert.Equal(t, 2, total)

		ids := []uuid.UUID{users[0].ID, users[1].ID}
		assert.ElementsMatch(t, []uuid.UUID{active1.ID, active2.ID}, ids)
		assert.NotContains(t, ids, deleted.ID)
	})

	t.Run("success - returns users with their roles", func(t *testing.T) {
		defer truncateAll(t)

		u1 := createTestUser(t, ctx, repo)
		u2 := createTestUser(t, ctx, repo)

		require.NoError(t, repo.AddRole(ctx, u1.ID, domain.RoleUser))
		require.NoError(t, repo.AddRole(ctx, u1.ID, domain.RoleAdmin))
		require.NoError(t, repo.AddRole(ctx, u2.ID, domain.RoleUser))

		users, _, err := repo.List(ctx, domain.ListUsersFilter{Limit: 10, Offset: 0})
		require.NoError(t, err)
		require.Len(t, users, 2)

		byID := make(map[uuid.UUID]*domain.User)
		for _, u := range users {
			byID[u.ID] = u
		}

		assert.ElementsMatch(t,
			[]domain.Role{domain.RoleUser, domain.RoleAdmin},
			byID[u1.ID].Roles,
		)
		assert.ElementsMatch(t,
			[]domain.Role{domain.RoleUser},
			byID[u2.ID].Roles,
		)
	})

	t.Run("success - users without roles return empty roles slice, not nil", func(t *testing.T) {
		defer truncateAll(t)

		createTestUser(t, ctx, repo)

		users, _, err := repo.List(ctx, domain.ListUsersFilter{Limit: 10, Offset: 0})
		require.NoError(t, err)
		require.Len(t, users, 1)

		assert.NotNil(t, users[0].Roles)
		assert.Empty(t, users[0].Roles)
	})
}
