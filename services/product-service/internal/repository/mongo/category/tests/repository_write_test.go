//go:build integration

package repository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/testutils"
	"github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/domain"
	productMongo "github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/infra/mongodb"
	categoryReadRepo "github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/repository/mongo/category/read"
	categoryWriteRepo "github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/repository/mongo/category/write"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/mongo"
)

var testDB *testutils.TestMongoDB

func TestMain(m *testing.M) {
	ctx := context.Background()

	db, err := testutils.SetupTestMongoDBShared(ctx, productMongo.SetupIndexes)
	if err != nil {
		panic(err)
	}
	testDB = db
	defer func() { _ = testDB.Close(ctx) }()

	code := m.Run()
	os.Exit(code)
}

func getTestDatabase(t *testing.T) *mongo.Database {
	t.Helper()
	return testDB.Client.Database(testDB.DBName)
}

func createTestCategory(t *testing.T, ctx context.Context, repo *categoryWriteRepo.CategoryWriteRepo) *domain.Category {
	t.Helper()

	now := time.Now().UTC()
	category := &domain.Category{
		Name:        "Test Category",
		Slug:        "test-category-" + uuid.New().String()[:8],
		Description: "Test description",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	id, err := repo.Create(ctx, category)
	require.NoError(t, err)
	category.ID = id

	return category
}

func TestCategoryWriteRepo_Create(t *testing.T) {
	ctx := context.Background()
	db := getTestDatabase(t)
	repo := categoryWriteRepo.NewCategoryWriteRepo(db)

	t.Run("success - creates category", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "categories")

		now := time.Now().UTC()
		category := &domain.Category{
			Name:        "Electronics",
			Slug:        "electronics",
			Description: "Electronic devices and accessories",
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		id, err := repo.Create(ctx, category)
		require.NoError(t, err)
		assert.NotEmpty(t, id)

		readRepo := categoryReadRepo.NewCategoryReadRepo(db)
		found, err := readRepo.GetBySlug(ctx, "electronics")
		require.NoError(t, err)
		assert.Equal(t, id, found.ID)
		assert.Equal(t, "Electronics", found.Name)
		assert.Equal(t, "electronics", found.Slug)
		assert.Equal(t, "Electronic devices and accessories", found.Description)
		assert.WithinDuration(t, now, found.CreatedAt, time.Second)
		assert.WithinDuration(t, now, found.UpdatedAt, time.Second)
	})

	t.Run("error - duplicate slug", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "categories")

		category1 := &domain.Category{
			Name:      "First",
			Slug:      "unique-slug",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		_, err := repo.Create(ctx, category1)
		require.NoError(t, err)

		category2 := &domain.Category{
			Name:      "Second",
			Slug:      "unique-slug",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		_, err = repo.Create(ctx, category2)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrAlreadyExists)
	})

	t.Run("success - creates multiple categories with different slugs", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "categories")

		category1 := &domain.Category{
			Name:      "Category 1",
			Slug:      "category-1",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		category2 := &domain.Category{
			Name:      "Category 2",
			Slug:      "category-2",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		id1, err := repo.Create(ctx, category1)
		require.NoError(t, err)
		assert.NotEmpty(t, id1)

		id2, err := repo.Create(ctx, category2)
		require.NoError(t, err)
		assert.NotEmpty(t, id2)
		assert.NotEqual(t, id1, id2)
	})
}

func TestCategoryWriteRepo_Update(t *testing.T) {
	ctx := context.Background()
	db := getTestDatabase(t)
	repo := categoryWriteRepo.NewCategoryWriteRepo(db)

	t.Run("success - updates name only", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "categories")

		category := createTestCategory(t, ctx, repo)

		time.Sleep(10 * time.Millisecond)

		newName := "Updated Name"
		input := domain.UpdateCategoryInput{
			Name: &newName,
		}

		updated, err := repo.Update(ctx, category.Slug, input)
		require.NoError(t, err)
		assert.Equal(t, "Updated Name", updated.Name)
		assert.Equal(t, category.Description, updated.Description) // не изменилось
		assert.True(t, updated.UpdatedAt.After(category.UpdatedAt))
	})

	t.Run("success - updates description only", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "categories")

		category := createTestCategory(t, ctx, repo)

		newDesc := "New description"
		input := domain.UpdateCategoryInput{
			Description: &newDesc,
		}

		updated, err := repo.Update(ctx, category.Slug, input)
		require.NoError(t, err)
		assert.Equal(t, category.Name, updated.Name) // не изменилось
		assert.Equal(t, "New description", updated.Description)
	})

	t.Run("success - updates both name and description", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "categories")

		category := createTestCategory(t, ctx, repo)

		newName := "New Name"
		newDesc := "New Description"
		input := domain.UpdateCategoryInput{
			Name:        &newName,
			Description: &newDesc,
		}

		updated, err := repo.Update(ctx, category.Slug, input)
		require.NoError(t, err)
		assert.Equal(t, "New Name", updated.Name)
		assert.Equal(t, "New Description", updated.Description)
	})

	t.Run("success - updates with empty description", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "categories")

		category := createTestCategory(t, ctx, repo)

		emptyDesc := ""
		input := domain.UpdateCategoryInput{
			Description: &emptyDesc,
		}

		updated, err := repo.Update(ctx, category.Slug, input)
		require.NoError(t, err)
		assert.Equal(t, "", updated.Description)
	})

	t.Run("error - category not found", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "categories")

		newName := "Some Name"
		input := domain.UpdateCategoryInput{
			Name: &newName,
		}

		_, err := repo.Update(ctx, "non-existent-slug", input)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("error - cannot update deleted category", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "categories")

		category := createTestCategory(t, ctx, repo)

		err := repo.Delete(ctx, category.Slug)
		require.NoError(t, err)

		newName := "Updated Name"
		input := domain.UpdateCategoryInput{
			Name: &newName,
		}

		_, err = repo.Update(ctx, category.Slug, input)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestCategoryWriteRepo_Delete(t *testing.T) {
	ctx := context.Background()
	db := getTestDatabase(t)
	repo := categoryWriteRepo.NewCategoryWriteRepo(db)
	readRepo := categoryReadRepo.NewCategoryReadRepo(db)

	t.Run("success - soft deletes category", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "categories")

		category := createTestCategory(t, ctx, repo)

		err := repo.Delete(ctx, category.Slug)
		require.NoError(t, err)

		_, err = readRepo.GetBySlug(ctx, category.Slug)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("error - category not found", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "categories")

		err := repo.Delete(ctx, "non-existent-slug")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("error - cannot delete already deleted category", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "categories")

		category := createTestCategory(t, ctx, repo)

		err := repo.Delete(ctx, category.Slug)
		require.NoError(t, err)

		err = repo.Delete(ctx, category.Slug)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}
