//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/testutils"
	"github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/domain"
	productReadRepo "github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/repository/mongo/product/read"
	productWriteRepo "github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/repository/mongo/product/write"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductReadRepo_CountByCategory(t *testing.T) {
	ctx := context.Background()
	db := getTestDatabase(t)
	writeRepo := productWriteRepo.NewProductWriteRepo(db)
	readRepo := productReadRepo.NewProductReadRepo(db)

	t.Run("success - counts products in category", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.CategorySlug = "electronics"
		})
		createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.CategorySlug = "electronics"
		})
		createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.CategorySlug = "electronics"
		})

		createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.CategorySlug = "clothing"
		})

		count, err := readRepo.CountByCategory(ctx, "electronics")
		require.NoError(t, err)
		assert.Equal(t, int64(3), count)
	})

	t.Run("success - returns 0 for empty category", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		count, err := readRepo.CountByCategory(ctx, "non-existent-category")
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})

	t.Run("success - does not count deleted products", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.CategorySlug = "smartphones"
		})
		createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.CategorySlug = "smartphones"
		})

		deletedProduct := createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.CategorySlug = "smartphones"
		})
		err := writeRepo.Delete(ctx, deletedProduct.ID)
		require.NoError(t, err)

		count, err := readRepo.CountByCategory(ctx, "smartphones")
		require.NoError(t, err)
		assert.Equal(t, int64(2), count, "should not count deleted product")
	})

	t.Run("success - does not count unpublished products", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.CategorySlug = "books"
			p.Published = true
		})

		createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.CategorySlug = "books"
			p.Published = false
		})

		count, err := readRepo.CountByCategory(ctx, "books")
		require.NoError(t, err)
		assert.Equal(t, int64(2), count) // считает оба
	})
}

func TestProductReadRepo_GetByID(t *testing.T) {
	ctx := context.Background()
	db := getTestDatabase(t)
	writeRepo := productWriteRepo.NewProductWriteRepo(db)
	readRepo := productReadRepo.NewProductReadRepo(db)

	t.Run("success - finds product by ID", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		created := createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.Name = "iPhone 15 Pro"
			p.Price = 99990000
		})

		found, err := readRepo.GetByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, created.ID, found.ID)
		assert.Equal(t, "iPhone 15 Pro", found.Name)
		assert.Equal(t, int64(99990000), found.Price)
	})

	t.Run("error - product not found (invalid ID)", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		_, err := readRepo.GetByID(ctx, "invalid-object-id")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("error - product not found (non-existent ID)", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		_, err := readRepo.GetByID(ctx, "507f1f77bcf86cd799439011")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("error - does not return deleted product", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		product := createTestProduct(t, ctx, writeRepo)

		err := writeRepo.Delete(ctx, product.ID)
		require.NoError(t, err)

		_, err = readRepo.GetByID(ctx, product.ID)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestProductReadRepo_List(t *testing.T) {
	ctx := context.Background()
	db := getTestDatabase(t)
	writeRepo := productWriteRepo.NewProductWriteRepo(db)
	readRepo := productReadRepo.NewProductReadRepo(db)

	t.Run("success - returns all published products", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.Published = true
		})
		createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.Published = true
		})

		filter := domain.ListProductsFilter{
			Sort:  domain.SortLatest,
			Limit: 20,
		}

		products, cursor, err := readRepo.List(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, products, 2)
		assert.Nil(t, cursor, "no next cursor when all results fit")
	})

	t.Run("success - does not return unpublished products", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.Published = true
		})
		createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.Published = false // неопубликованный
		})

		filter := domain.ListProductsFilter{
			Sort:  domain.SortLatest,
			Limit: 20,
		}

		products, _, err := readRepo.List(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, products, 1, "should return only published products")
	})

	t.Run("success - does not return deleted products", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		active := createTestProduct(t, ctx, writeRepo)
		deleted := createTestProduct(t, ctx, writeRepo)

		err := writeRepo.Delete(ctx, deleted.ID)
		require.NoError(t, err)

		filter := domain.ListProductsFilter{
			Sort:  domain.SortLatest,
			Limit: 20,
		}

		products, _, err := readRepo.List(ctx, filter)
		require.NoError(t, err)
		require.Len(t, products, 1)
		assert.Equal(t, active.ID, products[0].ID)
	})

	t.Run("success - filters by category", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.CategorySlug = "smartphones"
		})
		createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.CategorySlug = "smartphones"
		})
		createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.CategorySlug = "laptops"
		})

		categorySlug := "smartphones"
		filter := domain.ListProductsFilter{
			CategorySlug: &categorySlug,
			Sort:         domain.SortLatest,
			Limit:        20,
		}

		products, _, err := readRepo.List(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, products, 2)
	})

	t.Run("success - filters by price range", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.Price = 5000
		})
		createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.Price = 15000
		})
		createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.Price = 25000
		})

		priceMin := int64(10000)
		priceMax := int64(20000)
		filter := domain.ListProductsFilter{
			PriceMin: &priceMin,
			PriceMax: &priceMax,
			Sort:     domain.SortLatest,
			Limit:    20,
		}

		products, _, err := readRepo.List(ctx, filter)
		require.NoError(t, err)
		require.Len(t, products, 1)
		assert.Equal(t, int64(15000), products[0].Price)
	})

	t.Run("success - sort by latest (createdAt DESC)", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		time.Sleep(10 * time.Millisecond)
		p1 := createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.CreatedAt = time.Now().Add(-2 * time.Minute)
		})

		time.Sleep(10 * time.Millisecond)
		p2 := createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.CreatedAt = time.Now().Add(-1 * time.Minute)
		})

		time.Sleep(10 * time.Millisecond)
		p3 := createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.CreatedAt = time.Now()
		})

		filter := domain.ListProductsFilter{
			Sort:  domain.SortLatest,
			Limit: 20,
		}

		products, _, err := readRepo.List(ctx, filter)
		require.NoError(t, err)
		require.Len(t, products, 3)

		assert.Equal(t, p3.ID, products[0].ID)
		assert.Equal(t, p2.ID, products[1].ID)
		assert.Equal(t, p1.ID, products[2].ID)
	})

	t.Run("success - sort by price ASC", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		p1 := createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.Price = 30000
		})
		p2 := createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.Price = 10000
		})
		p3 := createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.Price = 20000
		})

		filter := domain.ListProductsFilter{
			Sort:  domain.SortPriceAsc,
			Limit: 20,
		}

		products, _, err := readRepo.List(ctx, filter)
		require.NoError(t, err)
		require.Len(t, products, 3)

		assert.Equal(t, p2.ID, products[0].ID)
		assert.Equal(t, p3.ID, products[1].ID)
		assert.Equal(t, p1.ID, products[2].ID)
	})

	t.Run("success - sort by price DESC", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		p1 := createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.Price = 10000
		})
		p2 := createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.Price = 30000
		})
		p3 := createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.Price = 20000
		})

		filter := domain.ListProductsFilter{
			Sort:  domain.SortPriceDesc,
			Limit: 20,
		}

		products, _, err := readRepo.List(ctx, filter)
		require.NoError(t, err)
		require.Len(t, products, 3)

		assert.Equal(t, p2.ID, products[0].ID)
		assert.Equal(t, p3.ID, products[1].ID)
		assert.Equal(t, p1.ID, products[2].ID)
	})

	t.Run("success - cursor pagination (SortLatest)", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		for i := 0; i < 5; i++ {
			createTestProduct(t, ctx, writeRepo)
			time.Sleep(10 * time.Millisecond)
		}

		filter := domain.ListProductsFilter{
			Sort:  domain.SortLatest,
			Limit: 2,
		}

		page1, cursor1, err := readRepo.List(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, page1, 2)
		require.NotNil(t, cursor1, "should have next cursor")

		filter.Cursor = cursor1
		page2, cursor2, err := readRepo.List(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, page2, 2)
		require.NotNil(t, cursor2)

		filter.Cursor = cursor2
		page3, cursor3, err := readRepo.List(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, page3, 1)
		assert.Nil(t, cursor3, "no more pages")

		allIDs := make(map[string]bool)
		for _, p := range append(append(page1, page2...), page3...) {
			assert.False(t, allIDs[p.ID], "duplicate product ID: %s", p.ID)
			allIDs[p.ID] = true
		}
	})

	t.Run("success - cursor pagination (SortPriceAsc)", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		createTestProduct(t, ctx, writeRepo, func(p *domain.Product) { p.Price = 10000 })
		createTestProduct(t, ctx, writeRepo, func(p *domain.Product) { p.Price = 20000 })
		createTestProduct(t, ctx, writeRepo, func(p *domain.Product) { p.Price = 30000 })

		filter := domain.ListProductsFilter{
			Sort:  domain.SortPriceAsc,
			Limit: 2,
		}

		page1, cursor, err := readRepo.List(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, page1, 2)
		assert.Equal(t, int64(10000), page1[0].Price)
		assert.Equal(t, int64(20000), page1[1].Price)
		require.NotNil(t, cursor)

		filter.Cursor = cursor
		page2, _, err := readRepo.List(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, page2, 1)
		assert.Equal(t, int64(30000), page2[0].Price)
	})

	t.Run("success - text search", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.Name = "iPhone 15 Pro"
			p.Description = strPtr("Latest Apple smartphone")
		})
		createTestProduct(t, ctx, writeRepo, func(p *domain.Product) {
			p.Name = "Samsung Galaxy S24"
			p.Description = strPtr("Android flagship")
		})

		searchTerm := "iPhone"
		filter := domain.ListProductsFilter{
			Search: &searchTerm,
			Limit:  20,
		}

		products, _, err := readRepo.List(ctx, filter)
		require.NoError(t, err)
		require.Len(t, products, 1)
		assert.Contains(t, products[0].Name, "iPhone")
	})

	t.Run("success - returns empty list when no matches", func(t *testing.T) {
		defer testutils.TruncateCollections(t, db, "products")

		categorySlug := "non-existent-category"
		filter := domain.ListProductsFilter{
			CategorySlug: &categorySlug,
			Sort:         domain.SortLatest,
			Limit:        20,
		}

		products, cursor, err := readRepo.List(ctx, filter)
		require.NoError(t, err)
		assert.Empty(t, products)
		assert.Nil(t, cursor)
	})
}
