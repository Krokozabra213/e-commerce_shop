//go:build integration

package repository_test

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/testutils"
	"github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/domain"
	productMongo "github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/infra/mongodb"
	productReadRepo "github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/repository/mongo/product/read"
	productWriteRepo "github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/repository/mongo/product/write"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	testDB         *testutils.TestMongoDB
	productCounter int64
)

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

func createTestProduct(t *testing.T, ctx context.Context, repo *productWriteRepo.ProductWriteRepo, opts ...func(*domain.Product)) *domain.Product {
	t.Helper()

	counter := atomic.AddInt64(&productCounter, 1)
	now := time.Now().UTC()

	product := &domain.Product{
		Name:         fmt.Sprintf("Test Product %d", counter),
		Description:  strPtr("Test description"),
		Price:        10000,
		CategorySlug: "test-category",
		Published:    true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	for _, opt := range opts {
		opt(product)
	}

	id, err := repo.Create(ctx, product)
	require.NoError(t, err)
	product.ID = id

	return product
}

func strPtr(s string) *string {
	return &s
}

func int64Ptr(i int64) *int64 {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}

func TestProductWriteRepo_Create(t *testing.T) {
	ctx := context.Background()
	db := getTestDatabase(t)
	writeRepo := productWriteRepo.NewProductWriteRepo(db)
	readRepo := productReadRepo.NewProductReadRepo(db)

	t.Run("success - creates product with all fields", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		now := time.Now().UTC()
		product := &domain.Product{
			Name:         "iPhone 15 Pro",
			Description:  strPtr("Latest Apple smartphone"),
			Price:        99990000,
			CategorySlug: "smartphones",
			Published:    true,
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		id, err := writeRepo.Create(ctx, product)
		require.NoError(t, err)
		assert.NotEmpty(t, id)

		found, err := readRepo.GetByID(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, id, found.ID)
		assert.Equal(t, "iPhone 15 Pro", found.Name)
		require.NotNil(t, found.Description)
		assert.Equal(t, "Latest Apple smartphone", *found.Description)
		assert.Equal(t, int64(99990000), found.Price)
		assert.Equal(t, "smartphones", found.CategorySlug)
		assert.True(t, found.Published)
		assert.WithinDuration(t, now, found.CreatedAt, time.Second)
		assert.WithinDuration(t, now, found.UpdatedAt, time.Second)
		assert.Nil(t, found.DeletedAt)
	})

	t.Run("success - creates product without description", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		product := &domain.Product{
			Name:         "Simple Product",
			Description:  nil,
			Price:        5000,
			CategorySlug: "misc",
			Published:    false,
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}

		id, err := writeRepo.Create(ctx, product)
		require.NoError(t, err)
		assert.NotEmpty(t, id)

		found, err := readRepo.GetByID(ctx, id)
		require.NoError(t, err)
		assert.Nil(t, found.Description)
		assert.False(t, found.Published)
	})

	t.Run("success - creates product with empty description", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		product := &domain.Product{
			Name:         "Product with empty desc",
			Description:  strPtr(""),
			Price:        1000,
			CategorySlug: "test",
			Published:    true,
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}

		id, err := writeRepo.Create(ctx, product)
		require.NoError(t, err)

		found, err := readRepo.GetByID(ctx, id)
		require.NoError(t, err)
		require.NotNil(t, found.Description)
		assert.Equal(t, "", *found.Description)
	})

	t.Run("success - creates multiple products with unique IDs", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		product1 := createTestProduct(t, ctx, writeRepo)
		product2 := createTestProduct(t, ctx, writeRepo)
		product3 := createTestProduct(t, ctx, writeRepo)

		assert.NotEqual(t, product1.ID, product2.ID)
		assert.NotEqual(t, product2.ID, product3.ID)
		assert.NotEqual(t, product1.ID, product3.ID)
	})

	t.Run("success - creates product with zero price", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		product := &domain.Product{
			Name:         "Free Product",
			Price:        0,
			CategorySlug: "free",
			Published:    true,
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}

		id, err := writeRepo.Create(ctx, product)
		require.NoError(t, err)

		found, err := readRepo.GetByID(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, int64(0), found.Price)
	})
}

func TestProductWriteRepo_Update(t *testing.T) {
	ctx := context.Background()
	db := getTestDatabase(t)
	writeRepo := productWriteRepo.NewProductWriteRepo(db)

	t.Run("success - updates name only", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		product := createTestProduct(t, ctx, writeRepo)
		originalPrice := product.Price
		originalDesc := product.Description
		originalUpdatedAt := product.UpdatedAt

		time.Sleep(10 * time.Millisecond)

		newName := "Updated Name"
		input := &domain.UpdateProductInput{
			Name: &newName,
		}

		updated, err := writeRepo.Update(ctx, product.ID, input)
		require.NoError(t, err)
		assert.Equal(t, "Updated Name", updated.Name)
		assert.Equal(t, originalPrice, updated.Price)
		assert.Equal(t, originalDesc, updated.Description)
		assert.True(t, updated.UpdatedAt.After(originalUpdatedAt))
	})

	t.Run("success - updates description only", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		product := createTestProduct(t, ctx, writeRepo)
		originalName := product.Name

		newDesc := "New description text"
		input := &domain.UpdateProductInput{
			Description: &newDesc,
		}

		updated, err := writeRepo.Update(ctx, product.ID, input)
		require.NoError(t, err)
		assert.Equal(t, originalName, updated.Name)
		require.NotNil(t, updated.Description)
		assert.Equal(t, "New description text", *updated.Description)
	})

	t.Run("success - updates price only", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		product := createTestProduct(t, ctx, writeRepo)
		originalName := product.Name

		newPrice := int64(50000)
		input := &domain.UpdateProductInput{
			Price: &newPrice,
		}

		updated, err := writeRepo.Update(ctx, product.ID, input)
		require.NoError(t, err)
		assert.Equal(t, originalName, updated.Name)
		assert.Equal(t, int64(50000), updated.Price)
	})

	t.Run("success - updates categorySlug only", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		product := createTestProduct(t, ctx, writeRepo)

		newCategory := "new-category"
		input := &domain.UpdateProductInput{
			CategorySlug: &newCategory,
		}

		updated, err := writeRepo.Update(ctx, product.ID, input)
		require.NoError(t, err)
		assert.Equal(t, "new-category", updated.CategorySlug)
	})

	t.Run("success - updates published only", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		product := createTestProduct(t, ctx, writeRepo)
		assert.True(t, product.Published)

		newPublished := false
		input := &domain.UpdateProductInput{
			Published: &newPublished,
		}

		updated, err := writeRepo.Update(ctx, product.ID, input)
		require.NoError(t, err)
		assert.False(t, updated.Published)
	})

	t.Run("success - updates all fields at once", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		product := createTestProduct(t, ctx, writeRepo)

		newName := "Completely New Name"
		newDesc := "Completely new description"
		newPrice := int64(20000)
		newCategory := "new-category"
		newPublished := false

		input := &domain.UpdateProductInput{
			Name:         &newName,
			Description:  &newDesc,
			Price:        &newPrice,
			CategorySlug: &newCategory,
			Published:    &newPublished,
		}

		updated, err := writeRepo.Update(ctx, product.ID, input)
		require.NoError(t, err)
		assert.Equal(t, "Completely New Name", updated.Name)
		assert.Equal(t, "Completely new description", *updated.Description)
		assert.Equal(t, int64(20000), updated.Price)
		assert.Equal(t, "new-category", updated.CategorySlug)
		assert.False(t, updated.Published)
	})

	t.Run("success - clears description (sets to empty string)", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		product := createTestProduct(t, ctx, writeRepo)
		require.NotNil(t, product.Description)
		assert.NotEqual(t, "", *product.Description)

		emptyDesc := ""
		input := &domain.UpdateProductInput{
			Description: &emptyDesc,
		}

		updated, err := writeRepo.Update(ctx, product.ID, input)
		require.NoError(t, err)
		require.NotNil(t, updated.Description)
		assert.Equal(t, "", *updated.Description)
	})

	t.Run("success - sets description to nil by omitting it", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		product := createTestProduct(t, ctx, writeRepo)
		originalDesc := product.Description

		newName := "New Name"
		input := &domain.UpdateProductInput{
			Name: &newName,
		}

		updated, err := writeRepo.Update(ctx, product.ID, input)
		require.NoError(t, err)
		assert.Equal(t, "New Name", updated.Name)
		assert.Equal(t, originalDesc, updated.Description) // не изменилось
	})

	t.Run("success - updates price to zero", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		product := createTestProduct(t, ctx, writeRepo)
		assert.NotEqual(t, int64(0), product.Price)

		zeroPrice := int64(0)
		input := &domain.UpdateProductInput{
			Price: &zeroPrice,
		}

		updated, err := writeRepo.Update(ctx, product.ID, input)
		require.NoError(t, err)
		assert.Equal(t, int64(0), updated.Price)
	})

	t.Run("error - product not found (invalid ID)", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		newName := "Test"
		input := &domain.UpdateProductInput{
			Name: &newName,
		}

		_, err := writeRepo.Update(ctx, "invalid-object-id", input)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("error - product not found (non-existent ID)", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		newName := "Test"
		input := &domain.UpdateProductInput{
			Name: &newName,
		}

		_, err := writeRepo.Update(ctx, "507f1f77bcf86cd799439011", input)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("error - cannot update deleted product", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		product := createTestProduct(t, ctx, writeRepo)

		err := writeRepo.Delete(ctx, product.ID)
		require.NoError(t, err)

		newName := "Updated"
		input := &domain.UpdateProductInput{
			Name: &newName,
		}

		_, err = writeRepo.Update(ctx, product.ID, input)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestProductWriteRepo_Delete(t *testing.T) {
	ctx := context.Background()
	db := getTestDatabase(t)
	writeRepo := productWriteRepo.NewProductWriteRepo(db)
	readRepo := productReadRepo.NewProductReadRepo(db)

	t.Run("success - soft deletes product", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		product := createTestProduct(t, ctx, writeRepo)

		err := writeRepo.Delete(ctx, product.ID)
		require.NoError(t, err)

		_, err = readRepo.GetByID(ctx, product.ID)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("success - deletes multiple products independently", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		product1 := createTestProduct(t, ctx, writeRepo)
		product2 := createTestProduct(t, ctx, writeRepo)

		err := writeRepo.Delete(ctx, product1.ID)
		require.NoError(t, err)

		found, err := readRepo.GetByID(ctx, product2.ID)
		require.NoError(t, err)
		assert.Equal(t, product2.ID, found.ID)
	})

	t.Run("error - product not found (invalid ID)", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		err := writeRepo.Delete(ctx, "invalid-object-id")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("error - product not found (non-existent ID)", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		err := writeRepo.Delete(ctx, "507f1f77bcf86cd799439011")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("error - cannot delete already deleted product", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		product := createTestProduct(t, ctx, writeRepo)

		err := writeRepo.Delete(ctx, product.ID)
		require.NoError(t, err)

		err = writeRepo.Delete(ctx, product.ID)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestProductWriteRepo_TogglePublish(t *testing.T) {
	ctx := context.Background()
	db := getTestDatabase(t)
	writeRepo := productWriteRepo.NewProductWriteRepo(db)
	readRepo := productReadRepo.NewProductReadRepo(db)

	t.Run("success - toggles published from true to false", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		product := createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.Published = true
		})
		assert.True(t, product.Published)

		time.Sleep(10 * time.Millisecond)

		updated, err := writeRepo.TogglePublish(ctx, product.ID)
		require.NoError(t, err)
		assert.False(t, updated.Published)
		assert.False(t, updated.UpdatedAt.Before(product.UpdatedAt),
			"updatedAt should not be before original: %v vs %v",
			updated.UpdatedAt, product.UpdatedAt)
	})

	t.Run("success - toggles published from false to true", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		product := createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.Published = false
		})
		assert.False(t, product.Published)

		updated, err := writeRepo.TogglePublish(ctx, product.ID)
		require.NoError(t, err)
		assert.True(t, updated.Published)
	})

	t.Run("success - toggles multiple times correctly", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		product := createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.Published = true
		})

		updated1, err := writeRepo.TogglePublish(ctx, product.ID)
		require.NoError(t, err)
		assert.False(t, updated1.Published)

		updated2, err := writeRepo.TogglePublish(ctx, product.ID)
		require.NoError(t, err)
		assert.True(t, updated2.Published)

		updated3, err := writeRepo.TogglePublish(ctx, product.ID)
		require.NoError(t, err)
		assert.False(t, updated3.Published)

		updated4, err := writeRepo.TogglePublish(ctx, product.ID)
		require.NoError(t, err)
		assert.True(t, updated4.Published)
	})

	t.Run("success - updates updatedAt timestamp", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		product := createTestProduct(t, ctx, writeRepo)
		originalUpdatedAt := product.UpdatedAt

		time.Sleep(10 * time.Millisecond)

		updated, err := writeRepo.TogglePublish(ctx, product.ID)
		require.NoError(t, err)
		assert.True(t, updated.UpdatedAt.After(originalUpdatedAt))
	})

	t.Run("success - does not affect other fields", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		product := createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.Name = "Original Name"
			p.Price = 12345
			p.CategorySlug = "original-category"
		})

		updated, err := writeRepo.TogglePublish(ctx, product.ID)
		require.NoError(t, err)
		assert.Equal(t, "Original Name", updated.Name)
		assert.Equal(t, int64(12345), updated.Price)
		assert.Equal(t, "original-category", updated.CategorySlug)
		assert.NotEqual(t, product.Published, updated.Published) // только published изменился
	})

	t.Run("success - result is immediately visible in read repo", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		product := createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.Published = true
		})

		updated, err := writeRepo.TogglePublish(ctx, product.ID)
		require.NoError(t, err)
		assert.False(t, updated.Published)

		filter := domain.ListProductsFilter{
			Limit: 10,
		}
		products, _, err := readRepo.List(ctx, filter)
		require.NoError(t, err)

		for _, p := range products {
			assert.NotEqual(t, product.ID, p.ID)
		}
	})

	t.Run("error - product not found (invalid ID)", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		_, err := writeRepo.TogglePublish(ctx, "invalid-object-id")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("error - product not found (non-existent ID)", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		_, err := writeRepo.TogglePublish(ctx, "507f1f77bcf86cd799439011")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("error - cannot toggle deleted product", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		product := createTestProduct(t, ctx, writeRepo)

		err := writeRepo.Delete(ctx, product.ID)
		require.NoError(t, err)

		_, err = writeRepo.TogglePublish(ctx, product.ID)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestProductWriteRepo_GetPricesByIDs(t *testing.T) {
	ctx := context.Background()
	db := getTestDatabase(t)
	writeRepo := productWriteRepo.NewProductWriteRepo(db)

	t.Run("success - returns prices for multiple products", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		p1 := createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.Price = 15000
		})
		p2 := createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.Price = 25000
		})
		p3 := createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.Price = 99999
		})

		prices, err := writeRepo.GetPricesByIDs(ctx, []string{p1.ID, p2.ID, p3.ID})
		require.NoError(t, err)
		require.Len(t, prices, 3)

		priceMap := make(map[string]int64, len(prices))
		for _, pp := range prices {
			priceMap[pp.ProductID] = pp.Price
		}

		assert.Equal(t, int64(15000), priceMap[p1.ID])
		assert.Equal(t, int64(25000), priceMap[p2.ID])
		assert.Equal(t, int64(99999), priceMap[p3.ID])
	})

	t.Run("success - returns empty slice for empty input", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		prices, err := writeRepo.GetPricesByIDs(ctx, []string{})
		require.NoError(t, err)
		assert.Nil(t, prices)
	})

	t.Run("success - returns empty slice for nil input", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		prices, err := writeRepo.GetPricesByIDs(ctx, nil)
		require.NoError(t, err)
		assert.Nil(t, prices)
	})

	t.Run("success - skips non-existent IDs", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		p1 := createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.Price = 42000
		})

		fakeID1 := primitive.NewObjectID().Hex()
		fakeID2 := primitive.NewObjectID().Hex()

		prices, err := writeRepo.GetPricesByIDs(ctx, []string{p1.ID, fakeID1, fakeID2})
		require.NoError(t, err)
		require.Len(t, prices, 1)
		assert.Equal(t, p1.ID, prices[0].ProductID)
		assert.Equal(t, int64(42000), prices[0].Price)
	})

	t.Run("success - skips deleted products", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		p1 := createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.Price = 10000
		})
		p2 := createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.Price = 20000
		})

		oid, _ := primitive.ObjectIDFromHex(p2.ID)
		_, err := db.Collection("products").UpdateOne(ctx,
			bson.M{"_id": oid},
			bson.M{"$set": bson.M{"deletedAt": time.Now().UTC()}},
		)
		require.NoError(t, err)

		prices, err := writeRepo.GetPricesByIDs(ctx, []string{p1.ID, p2.ID})
		require.NoError(t, err)
		require.Len(t, prices, 1)
		assert.Equal(t, p1.ID, prices[0].ProductID)
		assert.Equal(t, int64(10000), prices[0].Price)
	})

	t.Run("success - returns empty for all non-existent IDs", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		fakeIDs := []string{
			primitive.NewObjectID().Hex(),
			primitive.NewObjectID().Hex(),
		}

		prices, err := writeRepo.GetPricesByIDs(ctx, fakeIDs)
		require.NoError(t, err)
		assert.Empty(t, prices)
	})

	t.Run("success - skips invalid hex IDs", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		p1 := createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.Price = 77000
		})

		prices, err := writeRepo.GetPricesByIDs(ctx, []string{p1.ID, "not-a-valid-hex", "also-bad"})
		require.NoError(t, err)
		require.Len(t, prices, 1)
		assert.Equal(t, p1.ID, prices[0].ProductID)
		assert.Equal(t, int64(77000), prices[0].Price)
	})

	t.Run("success - handles single product", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		p1 := createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.Price = 12345
		})

		prices, err := writeRepo.GetPricesByIDs(ctx, []string{p1.ID})
		require.NoError(t, err)
		require.Len(t, prices, 1)
		assert.Equal(t, p1.ID, prices[0].ProductID)
		assert.Equal(t, int64(12345), prices[0].Price)
	})
}
