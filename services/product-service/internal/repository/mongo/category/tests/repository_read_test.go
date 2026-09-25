//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/testutils"
	"github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/domain"
	categoryReadRepo "github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/repository/mongo/category/read"
	categoryWriteRepo "github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/repository/mongo/category/write"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCategoryReadRepo_GetBySlug(t *testing.T) {
	ctx := context.Background()
	db := getTestDatabase(t)
	writeRepo := categoryWriteRepo.NewCategoryWriteRepo(db)
	readRepo := categoryReadRepo.NewCategoryReadRepo(db)

	t.Run("success - finds category by slug", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "categories")

		created := createTestCategory(t, ctx, writeRepo)

		found, err := readRepo.GetBySlug(ctx, created.Slug)
		require.NoError(t, err)
		assert.Equal(t, created.ID, found.ID)
		assert.Equal(t, created.Name, found.Name)
		assert.Equal(t, created.Slug, found.Slug)
		assert.Equal(t, created.Description, found.Description)
	})

	t.Run("error - category not found", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "categories")

		_, err := readRepo.GetBySlug(ctx, "non-existent-slug")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("error - does not return deleted category", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "categories")

		category := createTestCategory(t, ctx, writeRepo)

		err := writeRepo.Delete(ctx, category.Slug)
		require.NoError(t, err)

		_, err = readRepo.GetBySlug(ctx, category.Slug)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestCategoryReadRepo_List(t *testing.T) {
	ctx := context.Background()
	db := getTestDatabase(t)
	writeRepo := categoryWriteRepo.NewCategoryWriteRepo(db)
	readRepo := categoryReadRepo.NewCategoryReadRepo(db)

	t.Run("success - returns empty list when no categories", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "categories")

		categories, err := readRepo.List(ctx)
		require.NoError(t, err)
		assert.Empty(t, categories)
	})

	t.Run("success - returns all categories sorted by createdAt desc", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "categories")

		cat1 := &domain.Category{
			Name:      "Category 1",
			Slug:      "category-1",
			CreatedAt: time.Now().Add(-2 * time.Minute),
			UpdatedAt: time.Now().Add(-2 * time.Minute),
		}
		id1, err := writeRepo.Create(ctx, cat1)
		require.NoError(t, err)
		cat1.ID = id1

		time.Sleep(10 * time.Millisecond)

		cat2 := &domain.Category{
			Name:      "Category 2",
			Slug:      "category-2",
			CreatedAt: time.Now().Add(-1 * time.Minute),
			UpdatedAt: time.Now().Add(-1 * time.Minute),
		}
		id2, err := writeRepo.Create(ctx, cat2)
		require.NoError(t, err)
		cat2.ID = id2

		time.Sleep(10 * time.Millisecond)

		cat3 := &domain.Category{
			Name:      "Category 3",
			Slug:      "category-3",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		id3, err := writeRepo.Create(ctx, cat3)
		require.NoError(t, err)
		cat3.ID = id3

		categories, err := readRepo.List(ctx)
		require.NoError(t, err)
		require.Len(t, categories, 3)

		assert.Equal(t, cat3.ID, categories[0].ID)
		assert.Equal(t, cat2.ID, categories[1].ID)
		assert.Equal(t, cat1.ID, categories[2].ID)
	})

	t.Run("success - does not return deleted categories", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "categories")

		cat1 := createTestCategory(t, ctx, writeRepo)
		cat2 := createTestCategory(t, ctx, writeRepo)

		err := writeRepo.Delete(ctx, cat1.Slug)
		require.NoError(t, err)

		categories, err := readRepo.List(ctx)
		require.NoError(t, err)
		require.Len(t, categories, 1)
		assert.Equal(t, cat2.ID, categories[0].ID)
	})

	t.Run("success - returns categories with all fields", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "categories")

		now := time.Now().UTC()
		category := &domain.Category{
			Name:        "Full Category",
			Slug:        "full-category",
			Description: "Complete description",
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		id, err := writeRepo.Create(ctx, category)
		require.NoError(t, err)

		categories, err := readRepo.List(ctx)
		require.NoError(t, err)
		require.Len(t, categories, 1)

		found := categories[0]
		assert.Equal(t, id, found.ID)
		assert.Equal(t, "Full Category", found.Name)
		assert.Equal(t, "full-category", found.Slug)
		assert.Equal(t, "Complete description", found.Description)
		assert.WithinDuration(t, now, found.CreatedAt, time.Second)
		assert.WithinDuration(t, now, found.UpdatedAt, time.Second)
	})
}
