//go:build integration

package stockRepository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/testutils"
	tx_manager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/domain"
	stockRepository "github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/repository/postgres/stock"
	"github.com/Krokozabra213/e-commerce_shop/services/inventory-service/migrations"
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

func truncateAll(t *testing.T) {
	t.Helper()
	testutils.TruncateTables(t, testDB.Pool, "stocks", "outbox", "reservations")
}

func newStock(productID string, quantity int) *domain.Stock {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return &domain.Stock{
		ProductID:         productID,
		AvailableQuantity: quantity,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

func createTestStock(t *testing.T, ctx context.Context, repo *stockRepository.PostgresStockRepository, productID string, quantity int) *domain.Stock {
	t.Helper()
	stock := newStock(productID, quantity)
	require.NoError(t, repo.Create(ctx, stock))
	return stock
}

func TestPostgresStockRepository_Create(t *testing.T) {
	ctx := context.Background()
	repo := stockRepository.NewPostgresStockRepository(testDB.Pool)

	t.Run("success - creates stock without transaction", func(t *testing.T) {
		defer truncateAll(t)

		productID := uuid.New().String()
		stock := newStock(productID, 100)

		err := repo.Create(ctx, stock)
		require.NoError(t, err)

		found, err := repo.GetByProductID(ctx, productID)
		require.NoError(t, err)
		assert.Equal(t, stock.ProductID, found.ProductID)
		assert.Equal(t, stock.AvailableQuantity, found.AvailableQuantity)
		assert.WithinDuration(t, stock.CreatedAt, found.CreatedAt, time.Second)
		assert.WithinDuration(t, stock.UpdatedAt, found.UpdatedAt, time.Second)
	})

	t.Run("success - creates stock with zero quantity", func(t *testing.T) {
		defer truncateAll(t)

		productID := uuid.New().String()
		stock := newStock(productID, 0)

		err := repo.Create(ctx, stock)
		require.NoError(t, err)

		found, err := repo.GetByProductID(ctx, productID)
		require.NoError(t, err)
		assert.Equal(t, 0, found.AvailableQuantity)
	})

	t.Run("success - creates stock within transaction and commits", func(t *testing.T) {
		defer truncateAll(t)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		productID := uuid.New().String()
		stock := newStock(productID, 50)
		err = repo.Create(txCtx, stock)
		require.NoError(t, err)

		require.NoError(t, tx.Commit(ctx))

		found, err := repo.GetByProductID(ctx, productID)
		require.NoError(t, err)
		assert.Equal(t, stock.ProductID, found.ProductID)
	})

	t.Run("success - rollback within transaction does not persist stock", func(t *testing.T) {
		defer truncateAll(t)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		productID := uuid.New().String()
		stock := newStock(productID, 30)
		require.NoError(t, repo.Create(txCtx, stock))

		require.NoError(t, tx.Rollback(ctx))

		_, err = repo.GetByProductID(ctx, productID)
		require.ErrorIs(t, err, domain.NotFoundError)
	})

	t.Run("error - duplicate product_id", func(t *testing.T) {
		defer truncateAll(t)

		productID := uuid.New().String()
		stock1 := newStock(productID, 100)
		stock2 := newStock(productID, 200)

		require.NoError(t, repo.Create(ctx, stock1))

		err := repo.Create(ctx, stock2)
		require.ErrorIs(t, err, domain.AlreadyExistsError)
	})
}

func TestPostgresStockRepository_GetByProductID(t *testing.T) {
	ctx := context.Background()
	repo := stockRepository.NewPostgresStockRepository(testDB.Pool)

	t.Run("success - gets existing stock", func(t *testing.T) {
		defer truncateAll(t)

		productID := uuid.New().String()
		created := createTestStock(t, ctx, repo, productID, 75)

		found, err := repo.GetByProductID(ctx, productID)
		require.NoError(t, err)
		assert.Equal(t, created.ProductID, found.ProductID)
		assert.Equal(t, created.AvailableQuantity, found.AvailableQuantity)
		assert.WithinDuration(t, created.CreatedAt, found.CreatedAt, time.Second)
		assert.WithinDuration(t, created.UpdatedAt, found.UpdatedAt, time.Second)
	})

	t.Run("success - gets stock within transaction", func(t *testing.T) {
		defer truncateAll(t)

		productID := uuid.New().String()
		createTestStock(t, ctx, repo, productID, 60)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		found, err := repo.GetByProductID(txCtx, productID)
		require.NoError(t, err)
		assert.Equal(t, productID, found.ProductID)
	})

	t.Run("error - product not found", func(t *testing.T) {
		defer truncateAll(t)

		nonExistentID := uuid.New().String()

		_, err := repo.GetByProductID(ctx, nonExistentID)
		require.ErrorIs(t, err, domain.NotFoundError)
	})
}

func TestPostgresStockRepository_GetByProductIDs(t *testing.T) {
	ctx := context.Background()
	repo := stockRepository.NewPostgresStockRepository(testDB.Pool)

	t.Run("success - gets multiple stocks", func(t *testing.T) {
		defer truncateAll(t)

		productID1 := uuid.New().String()
		productID2 := uuid.New().String()
		productID3 := uuid.New().String()

		createTestStock(t, ctx, repo, productID1, 10)
		createTestStock(t, ctx, repo, productID2, 20)
		createTestStock(t, ctx, repo, productID3, 30)

		quantities, err := repo.GetByProductIDs(ctx, []string{productID1, productID2, productID3})
		require.NoError(t, err)
		assert.Len(t, quantities, 3)
		assert.Equal(t, 10, quantities[productID1])
		assert.Equal(t, 20, quantities[productID2])
		assert.Equal(t, 30, quantities[productID3])
	})

	t.Run("success - returns only existing products", func(t *testing.T) {
		defer truncateAll(t)

		existingID := uuid.New().String()
		nonExistentID := uuid.New().String()

		createTestStock(t, ctx, repo, existingID, 50)

		quantities, err := repo.GetByProductIDs(ctx, []string{existingID, nonExistentID})
		require.NoError(t, err)
		assert.Len(t, quantities, 1)
		assert.Equal(t, 50, quantities[existingID])
		assert.NotContains(t, quantities, nonExistentID)
	})

	t.Run("success - empty list returns empty map", func(t *testing.T) {
		defer truncateAll(t)

		quantities, err := repo.GetByProductIDs(ctx, []string{})
		require.NoError(t, err)
		assert.Empty(t, quantities)
	})

	t.Run("success - gets stocks within transaction", func(t *testing.T) {
		defer truncateAll(t)

		productID1 := uuid.New().String()
		productID2 := uuid.New().String()
		createTestStock(t, ctx, repo, productID1, 15)
		createTestStock(t, ctx, repo, productID2, 25)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		quantities, err := repo.GetByProductIDs(txCtx, []string{productID1, productID2})
		require.NoError(t, err)
		assert.Len(t, quantities, 2)
	})

	t.Run("success - all non-existent products returns empty map", func(t *testing.T) {
		defer truncateAll(t)

		quantities, err := repo.GetByProductIDs(ctx, []string{uuid.New().String(), uuid.New().String()})
		require.NoError(t, err)
		assert.Empty(t, quantities)
	})
}

func TestPostgresStockRepository_IncreaseQuantity(t *testing.T) {
	ctx := context.Background()
	repo := stockRepository.NewPostgresStockRepository(testDB.Pool)

	t.Run("success - increases quantity", func(t *testing.T) {
		defer truncateAll(t)

		productID := uuid.New().String()
		createTestStock(t, ctx, repo, productID, 100)

		err := repo.IncreaseQuantity(ctx, productID, 50)
		require.NoError(t, err)

		stock, err := repo.GetByProductID(ctx, productID)
		require.NoError(t, err)
		assert.Equal(t, 150, stock.AvailableQuantity)
	})

	t.Run("success - increases from zero", func(t *testing.T) {
		defer truncateAll(t)

		productID := uuid.New().String()
		createTestStock(t, ctx, repo, productID, 0)

		err := repo.IncreaseQuantity(ctx, productID, 25)
		require.NoError(t, err)

		stock, err := repo.GetByProductID(ctx, productID)
		require.NoError(t, err)
		assert.Equal(t, 25, stock.AvailableQuantity)
	})

	t.Run("success - increases quantity within transaction and commits", func(t *testing.T) {
		defer truncateAll(t)

		productID := uuid.New().String()
		createTestStock(t, ctx, repo, productID, 40)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		err = repo.IncreaseQuantity(txCtx, productID, 10)
		require.NoError(t, err)

		require.NoError(t, tx.Commit(ctx))

		stock, err := repo.GetByProductID(ctx, productID)
		require.NoError(t, err)
		assert.Equal(t, 50, stock.AvailableQuantity)
	})

	t.Run("success - rollback within transaction does not persist increase", func(t *testing.T) {
		defer truncateAll(t)

		productID := uuid.New().String()
		createTestStock(t, ctx, repo, productID, 100)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		require.NoError(t, repo.IncreaseQuantity(txCtx, productID, 50))
		require.NoError(t, tx.Rollback(ctx))

		stock, err := repo.GetByProductID(ctx, productID)
		require.NoError(t, err)
		assert.Equal(t, 100, stock.AvailableQuantity)
	})

	t.Run("success - updates updated_at timestamp", func(t *testing.T) {
		defer truncateAll(t)

		productID := uuid.New().String()
		created := createTestStock(t, ctx, repo, productID, 100)

		time.Sleep(10 * time.Millisecond)

		err := repo.IncreaseQuantity(ctx, productID, 50)
		require.NoError(t, err)

		stock, err := repo.GetByProductID(ctx, productID)
		require.NoError(t, err)
		assert.True(t, stock.UpdatedAt.After(created.UpdatedAt))
	})

	t.Run("error - product not found", func(t *testing.T) {
		defer truncateAll(t)

		nonExistentID := uuid.New().String()

		err := repo.IncreaseQuantity(ctx, nonExistentID, 10)
		require.ErrorIs(t, err, domain.NotFoundError)
	})
}

func TestPostgresStockRepository_DecreaseQuantity(t *testing.T) {
	ctx := context.Background()
	repo := stockRepository.NewPostgresStockRepository(testDB.Pool)

	t.Run("success - decreases quantity", func(t *testing.T) {
		defer truncateAll(t)

		productID := uuid.New().String()
		createTestStock(t, ctx, repo, productID, 100)

		err := repo.DecreaseQuantity(ctx, productID, 30)
		require.NoError(t, err)

		stock, err := repo.GetByProductID(ctx, productID)
		require.NoError(t, err)
		assert.Equal(t, 70, stock.AvailableQuantity)
	})

	t.Run("success - decreases to zero", func(t *testing.T) {
		defer truncateAll(t)

		productID := uuid.New().String()
		createTestStock(t, ctx, repo, productID, 50)

		err := repo.DecreaseQuantity(ctx, productID, 50)
		require.NoError(t, err)

		stock, err := repo.GetByProductID(ctx, productID)
		require.NoError(t, err)
		assert.Equal(t, 0, stock.AvailableQuantity)
	})

	t.Run("success - decreases quantity within transaction and commits", func(t *testing.T) {
		defer truncateAll(t)

		productID := uuid.New().String()
		createTestStock(t, ctx, repo, productID, 100)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		err = repo.DecreaseQuantity(txCtx, productID, 25)
		require.NoError(t, err)

		require.NoError(t, tx.Commit(ctx))

		stock, err := repo.GetByProductID(ctx, productID)
		require.NoError(t, err)
		assert.Equal(t, 75, stock.AvailableQuantity)
	})

	t.Run("success - rollback within transaction does not persist decrease", func(t *testing.T) {
		defer truncateAll(t)

		productID := uuid.New().String()
		createTestStock(t, ctx, repo, productID, 100)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		require.NoError(t, repo.DecreaseQuantity(txCtx, productID, 40))
		require.NoError(t, tx.Rollback(ctx))

		stock, err := repo.GetByProductID(ctx, productID)
		require.NoError(t, err)
		assert.Equal(t, 100, stock.AvailableQuantity)
	})

	t.Run("success - updates updated_at timestamp", func(t *testing.T) {
		defer truncateAll(t)

		productID := uuid.New().String()
		created := createTestStock(t, ctx, repo, productID, 100)

		time.Sleep(10 * time.Millisecond)

		err := repo.DecreaseQuantity(ctx, productID, 20)
		require.NoError(t, err)

		stock, err := repo.GetByProductID(ctx, productID)
		require.NoError(t, err)
		assert.True(t, stock.UpdatedAt.After(created.UpdatedAt))
	})

	t.Run("error - product not found", func(t *testing.T) {
		defer truncateAll(t)

		nonExistentID := uuid.New().String()

		err := repo.DecreaseQuantity(ctx, nonExistentID, 10)
		require.ErrorIs(t, err, domain.NotFoundError)
	})

	t.Run("error - insufficient stock", func(t *testing.T) {
		defer truncateAll(t)

		productID := uuid.New().String()
		createTestStock(t, ctx, repo, productID, 30)

		err := repo.DecreaseQuantity(ctx, productID, 50)
		require.ErrorIs(t, err, domain.InsufficientStockError)
	})

	t.Run("error - cannot decrease from zero", func(t *testing.T) {
		defer truncateAll(t)

		productID := uuid.New().String()
		createTestStock(t, ctx, repo, productID, 0)

		err := repo.DecreaseQuantity(ctx, productID, 1)
		require.ErrorIs(t, err, domain.InsufficientStockError)
	})
}
