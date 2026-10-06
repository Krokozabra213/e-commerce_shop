//go:build integration

package orderRepository_tests

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/testutils"
	txmanager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/domain"
	orderRepository "github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/repository/postgres/order"
	"github.com/Krokozabra213/e-commerce_shop/services/order-service/migrations"
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
	testutils.TruncateTables(t, testDB.Pool, "order_items", "saga_state", "orders", "outbox", "inbox_events")
}

func newOrder(userID, idempotencyKey uuid.UUID, totalPrice int64) *domain.Order {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return &domain.Order{
		ID:             uuid.New(),
		UserID:         userID,
		IdempotencyKey: idempotencyKey,
		Status:         domain.OrderStatusNew,
		TotalPrice:     totalPrice,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func newOrderItem(orderID uuid.UUID, productID string, quantity int, price int64) domain.OrderItem {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return domain.OrderItem{
		ID:        uuid.New(),
		OrderID:   orderID,
		ProductID: productID,
		Quantity:  quantity,
		Price:     price,
		CreatedAt: now,
	}
}

func createTestOrder(t *testing.T, ctx context.Context, repo *orderRepository.PostgresOrderRepository, userID uuid.UUID, totalPrice int64) *domain.Order {
	t.Helper()
	order := newOrder(userID, uuid.New(), totalPrice)
	require.NoError(t, repo.Create(ctx, order))
	return order
}

func TestPostgresOrderRepository_Create(t *testing.T) {
	ctx := context.Background()
	repo := orderRepository.NewPostgresOrderRepository(testDB.Pool)

	t.Run("success - creates order without transaction", func(t *testing.T) {
		defer truncateAll(t)

		userID := uuid.New()
		idempotencyKey := uuid.New()
		order := newOrder(userID, idempotencyKey, 50000)

		err := repo.Create(ctx, order)
		require.NoError(t, err)

		found, err := repo.GetByID(ctx, order.ID)
		require.NoError(t, err)
		assert.Equal(t, order.ID, found.ID)
		assert.Equal(t, order.UserID, found.UserID)
		assert.Equal(t, order.IdempotencyKey, found.IdempotencyKey)
		assert.Equal(t, order.Status, found.Status)
		assert.Equal(t, order.TotalPrice, found.TotalPrice)
		assert.WithinDuration(t, order.CreatedAt, found.CreatedAt, time.Second)
		assert.WithinDuration(t, order.UpdatedAt, found.UpdatedAt, time.Second)
	})

	t.Run("success - creates order with zero price", func(t *testing.T) {
		defer truncateAll(t)

		order := newOrder(uuid.New(), uuid.New(), 0)

		err := repo.Create(ctx, order)
		require.NoError(t, err)

		found, err := repo.GetByID(ctx, order.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), found.TotalPrice)
	})

	t.Run("success - creates order within transaction and commits", func(t *testing.T) {
		defer truncateAll(t)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := txmanager.CtxWithTx(ctx, tx)

		order := newOrder(uuid.New(), uuid.New(), 100000)
		err = repo.Create(txCtx, order)
		require.NoError(t, err)

		require.NoError(t, tx.Commit(ctx))

		found, err := repo.GetByID(ctx, order.ID)
		require.NoError(t, err)
		assert.Equal(t, order.ID, found.ID)
	})

	t.Run("success - rollback within transaction does not persist order", func(t *testing.T) {
		defer truncateAll(t)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)

		txCtx := txmanager.CtxWithTx(ctx, tx)

		order := newOrder(uuid.New(), uuid.New(), 75000)
		require.NoError(t, repo.Create(txCtx, order))

		require.NoError(t, tx.Rollback(ctx))

		_, err = repo.GetByID(ctx, order.ID)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("error - duplicate idempotency_key", func(t *testing.T) {
		defer truncateAll(t)

		idempotencyKey := uuid.New()
		order1 := newOrder(uuid.New(), idempotencyKey, 50000)
		order2 := newOrder(uuid.New(), idempotencyKey, 60000)

		require.NoError(t, repo.Create(ctx, order1))

		err := repo.Create(ctx, order2)
		require.ErrorIs(t, err, domain.ErrAlreadyExists)
	})
}

func TestPostgresOrderRepository_GetByID(t *testing.T) {
	ctx := context.Background()
	repo := orderRepository.NewPostgresOrderRepository(testDB.Pool)

	t.Run("success - finds existing order", func(t *testing.T) {
		defer truncateAll(t)

		order := createTestOrder(t, ctx, repo, uuid.New(), 50000)

		found, err := repo.GetByID(ctx, order.ID)
		require.NoError(t, err)
		assert.Equal(t, order.ID, found.ID)
		assert.Equal(t, order.UserID, found.UserID)
		assert.Equal(t, order.Status, found.Status)
	})

	t.Run("error - order not found", func(t *testing.T) {
		defer truncateAll(t)

		nonExistentID := uuid.New()

		_, err := repo.GetByID(ctx, nonExistentID)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("success - reads within transaction", func(t *testing.T) {
		defer truncateAll(t)

		order := createTestOrder(t, ctx, repo, uuid.New(), 50000)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := txmanager.CtxWithTx(ctx, tx)

		found, err := repo.GetByID(txCtx, order.ID)
		require.NoError(t, err)
		assert.Equal(t, order.ID, found.ID)

		require.NoError(t, tx.Commit(ctx))
	})
}

func TestPostgresOrderRepository_GetByIdempotencyKey(t *testing.T) {
	ctx := context.Background()
	repo := orderRepository.NewPostgresOrderRepository(testDB.Pool)

	t.Run("success - finds order by idempotency key", func(t *testing.T) {
		defer truncateAll(t)

		idempotencyKey := uuid.New()
		order := newOrder(uuid.New(), idempotencyKey, 50000)
		require.NoError(t, repo.Create(ctx, order))

		found, err := repo.GetByIdempotencyKey(ctx, idempotencyKey)
		require.NoError(t, err)
		assert.Equal(t, order.ID, found.ID)
		assert.Equal(t, order.IdempotencyKey, found.IdempotencyKey)
	})

	t.Run("error - idempotency key not found", func(t *testing.T) {
		defer truncateAll(t)

		nonExistentKey := uuid.New()

		_, err := repo.GetByIdempotencyKey(ctx, nonExistentKey)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestPostgresOrderRepository_UpdateStatus(t *testing.T) {
	ctx := context.Background()
	repo := orderRepository.NewPostgresOrderRepository(testDB.Pool)

	t.Run("success - updates order status", func(t *testing.T) {
		defer truncateAll(t)

		order := createTestOrder(t, ctx, repo, uuid.New(), 50000)
		assert.Equal(t, domain.OrderStatusNew, order.Status)

		err := repo.UpdateStatus(ctx, order.ID, domain.OrderStatusReserved)
		require.NoError(t, err)

		updated, err := repo.GetByID(ctx, order.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.OrderStatusReserved, updated.Status)
		assert.True(t, updated.UpdatedAt.After(order.UpdatedAt))
	})

	t.Run("success - updates status within transaction", func(t *testing.T) {
		defer truncateAll(t)

		order := createTestOrder(t, ctx, repo, uuid.New(), 50000)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := txmanager.CtxWithTx(ctx, tx)

		err = repo.UpdateStatus(txCtx, order.ID, domain.OrderStatusPaid)
		require.NoError(t, err)

		require.NoError(t, tx.Commit(ctx))

		updated, err := repo.GetByID(ctx, order.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.OrderStatusPaid, updated.Status)
	})

	t.Run("success - rollback does not update status", func(t *testing.T) {
		defer truncateAll(t)

		order := createTestOrder(t, ctx, repo, uuid.New(), 50000)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)

		txCtx := txmanager.CtxWithTx(ctx, tx)

		require.NoError(t, repo.UpdateStatus(txCtx, order.ID, domain.OrderStatusCancelled))
		require.NoError(t, tx.Rollback(ctx))

		unchanged, err := repo.GetByID(ctx, order.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.OrderStatusNew, unchanged.Status)
	})

	t.Run("error - order not found", func(t *testing.T) {
		defer truncateAll(t)

		nonExistentID := uuid.New()

		err := repo.UpdateStatus(ctx, nonExistentID, domain.OrderStatusPaid)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestPostgresOrderRepository_CreateItems(t *testing.T) {
	ctx := context.Background()
	repo := orderRepository.NewPostgresOrderRepository(testDB.Pool)

	t.Run("success - creates single item", func(t *testing.T) {
		defer truncateAll(t)

		order := createTestOrder(t, ctx, repo, uuid.New(), 50000)

		items := []domain.OrderItem{
			newOrderItem(order.ID, "prod-1", 2, 25000),
		}

		err := repo.CreateItems(ctx, items)
		require.NoError(t, err)

		// Verify items were created
		var count int
		err = testDB.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM order_items WHERE order_id = $1", order.ID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("success - creates multiple items in batch", func(t *testing.T) {
		defer truncateAll(t)

		order := createTestOrder(t, ctx, repo, uuid.New(), 100000)

		items := []domain.OrderItem{
			newOrderItem(order.ID, "prod-1", 2, 25000),
			newOrderItem(order.ID, "prod-2", 1, 50000),
			newOrderItem(order.ID, "prod-3", 3, 10000),
		}

		err := repo.CreateItems(ctx, items)
		require.NoError(t, err)

		var count int
		err = testDB.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM order_items WHERE order_id = $1", order.ID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 3, count)
	})

	t.Run("success - creates items within transaction", func(t *testing.T) {
		defer truncateAll(t)

		order := createTestOrder(t, ctx, repo, uuid.New(), 50000)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := txmanager.CtxWithTx(ctx, tx)

		items := []domain.OrderItem{
			newOrderItem(order.ID, "prod-1", 5, 10000),
		}

		err = repo.CreateItems(txCtx, items)
		require.NoError(t, err)

		require.NoError(t, tx.Commit(ctx))

		var count int
		err = testDB.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM order_items WHERE order_id = $1", order.ID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("success - rollback does not persist items", func(t *testing.T) {
		defer truncateAll(t)

		order := createTestOrder(t, ctx, repo, uuid.New(), 50000)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)

		txCtx := txmanager.CtxWithTx(ctx, tx)

		items := []domain.OrderItem{
			newOrderItem(order.ID, "prod-1", 1, 50000),
		}

		require.NoError(t, repo.CreateItems(txCtx, items))
		require.NoError(t, tx.Rollback(ctx))

		var count int
		err = testDB.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM order_items WHERE order_id = $1", order.ID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("error - foreign key violation for non-existent order", func(t *testing.T) {
		defer truncateAll(t)

		nonExistentOrderID := uuid.New()

		items := []domain.OrderItem{
			newOrderItem(nonExistentOrderID, "prod-1", 1, 50000),
		}

		err := repo.CreateItems(ctx, items)
		require.Error(t, err)
	})

	t.Run("success - creates empty items slice (no-op)", func(t *testing.T) {
		defer truncateAll(t)

		order := createTestOrder(t, ctx, repo, uuid.New(), 0)

		items := []domain.OrderItem{}

		err := repo.CreateItems(ctx, items)
		require.NoError(t, err)

		var count int
		err = testDB.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM order_items WHERE order_id = $1", order.ID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}

func TestPostgresOrderRepository_ConcurrentIdempotency(t *testing.T) {
	ctx := context.Background()
	repo := orderRepository.NewPostgresOrderRepository(testDB.Pool)

	t.Run("concurrent creates with same idempotency key - one succeeds", func(t *testing.T) {
		defer truncateAll(t)

		idempotencyKey := uuid.New()
		userID := uuid.New()

		order1 := newOrder(userID, idempotencyKey, 50000)
		order2 := newOrder(userID, idempotencyKey, 60000)

		// Concurrent goroutines
		errChan := make(chan error, 2)

		go func() {
			errChan <- repo.Create(ctx, order1)
		}()

		go func() {
			errChan <- repo.Create(ctx, order2)
		}()

		err1 := <-errChan
		err2 := <-errChan

		if err1 == nil {
			require.ErrorIs(t, err2, domain.ErrAlreadyExists)
		} else if err2 == nil {
			require.ErrorIs(t, err1, domain.ErrAlreadyExists)
		} else {
			t.Fatal("both creates failed")
		}

		found, err := repo.GetByIdempotencyKey(ctx, idempotencyKey)
		require.NoError(t, err)
		assert.NotNil(t, found)
	})
}
