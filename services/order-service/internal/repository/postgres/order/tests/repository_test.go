//go:build integration

package orderRepository_tests

import (
	"context"
	"os"
	"strings"
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
		userID := uuid.New()
		order1 := newOrder(userID, idempotencyKey, 50000)
		order2 := newOrder(userID, idempotencyKey, 60000)

		require.NoError(t, repo.Create(ctx, order1))

		err := repo.Create(ctx, order2)
		require.ErrorIs(t, err, domain.ErrAlreadyExists)
	})

	t.Run("success - same idempotency_key for different users", func(t *testing.T) {
		defer truncateAll(t)

		idempotencyKey := uuid.New()
		order1 := newOrder(uuid.New(), idempotencyKey, 50000)
		order2 := newOrder(uuid.New(), idempotencyKey, 60000)

		require.NoError(t, repo.Create(ctx, order1))
		require.NoError(t, repo.Create(ctx, order2))

		var count int
		err := testDB.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM orders").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})
}

func TestPostgresOrderRepository_CreateIfAbsent(t *testing.T) {
	ctx := context.Background()
	repo := orderRepository.NewPostgresOrderRepository(testDB.Pool)

	t.Run("success - inserts new order", func(t *testing.T) {
		defer truncateAll(t)

		order := newOrder(uuid.New(), uuid.New(), 50000)

		inserted, err := repo.CreateIfAbsent(ctx, order)
		require.NoError(t, err)
		assert.True(t, inserted)

		found, err := repo.GetByID(ctx, order.ID)
		require.NoError(t, err)
		assert.Equal(t, order.ID, found.ID)
		assert.Equal(t, order.UserID, found.UserID)
		assert.Equal(t, order.IdempotencyKey, found.IdempotencyKey)
		assert.Equal(t, order.Status, found.Status)
		assert.Equal(t, order.TotalPrice, found.TotalPrice)
		assert.WithinDuration(t, order.CreatedAt, found.CreatedAt, time.Second)
	})

	t.Run("success - returns false on duplicate and keeps original order", func(t *testing.T) {
		defer truncateAll(t)

		userID := uuid.New()
		idempotencyKey := uuid.New()
		original := newOrder(userID, idempotencyKey, 50000)
		duplicate := newOrder(userID, idempotencyKey, 60000)

		inserted, err := repo.CreateIfAbsent(ctx, original)
		require.NoError(t, err)
		assert.True(t, inserted)

		inserted, err = repo.CreateIfAbsent(ctx, duplicate)
		require.NoError(t, err)
		assert.False(t, inserted)

		// Дубликат не должен ни появиться, ни перезаписать оригинал.
		var count int
		err = testDB.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM orders").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		found, err := repo.GetByUserAndIdempotencyKey(ctx, userID, idempotencyKey)
		require.NoError(t, err)
		assert.Equal(t, original.ID, found.ID)
		assert.Equal(t, int64(50000), found.TotalPrice)
	})

	t.Run("success - same idempotency key for different users inserts both", func(t *testing.T) {
		defer truncateAll(t)

		idempotencyKey := uuid.New()
		order1 := newOrder(uuid.New(), idempotencyKey, 50000)
		order2 := newOrder(uuid.New(), idempotencyKey, 60000)

		inserted, err := repo.CreateIfAbsent(ctx, order1)
		require.NoError(t, err)
		assert.True(t, inserted)

		inserted, err = repo.CreateIfAbsent(ctx, order2)
		require.NoError(t, err)
		assert.True(t, inserted)

		var count int
		err = testDB.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM orders").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("success - conflict does not abort ongoing transaction", func(t *testing.T) {
		defer truncateAll(t)

		userID := uuid.New()
		idempotencyKey := uuid.New()
		original := newOrder(userID, idempotencyKey, 50000)
		require.NoError(t, repo.Create(ctx, original))

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := txmanager.CtxWithTx(ctx, tx)

		inserted, err := repo.CreateIfAbsent(txCtx, newOrder(userID, idempotencyKey, 60000))
		require.NoError(t, err)
		assert.False(t, inserted)

		// Главная регрессия: после конфликта транзакция должна остаться рабочей.
		// Раньше здесь был SQLSTATE 25P02, и весь CreateOrder падал с 500.
		found, err := repo.GetByUserAndIdempotencyKey(txCtx, userID, idempotencyKey)
		require.NoError(t, err)
		assert.Equal(t, original.ID, found.ID)

		require.NoError(t, tx.Commit(ctx))
	})

	t.Run("success - inserts within transaction and commits", func(t *testing.T) {
		defer truncateAll(t)

		order := newOrder(uuid.New(), uuid.New(), 100000)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := txmanager.CtxWithTx(ctx, tx)

		inserted, err := repo.CreateIfAbsent(txCtx, order)
		require.NoError(t, err)
		assert.True(t, inserted)

		require.NoError(t, tx.Commit(ctx))

		found, err := repo.GetByID(ctx, order.ID)
		require.NoError(t, err)
		assert.Equal(t, order.ID, found.ID)
	})

	t.Run("success - rollback does not persist order", func(t *testing.T) {
		defer truncateAll(t)

		order := newOrder(uuid.New(), uuid.New(), 75000)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)

		txCtx := txmanager.CtxWithTx(ctx, tx)

		inserted, err := repo.CreateIfAbsent(txCtx, order)
		require.NoError(t, err)
		assert.True(t, inserted)

		require.NoError(t, tx.Rollback(ctx))

		_, err = repo.GetByID(ctx, order.ID)
		require.ErrorIs(t, err, domain.ErrNotFound)
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

func TestPostgresOrderRepository_GetByUserAndIdempotencyKey(t *testing.T) {
	ctx := context.Background()
	repo := orderRepository.NewPostgresOrderRepository(testDB.Pool)

	t.Run("success - finds own order", func(t *testing.T) {
		defer truncateAll(t)

		userID := uuid.New()
		idempotencyKey := uuid.New()
		order := newOrder(userID, idempotencyKey, 50000)
		require.NoError(t, repo.Create(ctx, order))

		found, err := repo.GetByUserAndIdempotencyKey(ctx, userID, idempotencyKey)
		require.NoError(t, err)
		assert.Equal(t, order.ID, found.ID)
		assert.Equal(t, userID, found.UserID)
		assert.Equal(t, idempotencyKey, found.IdempotencyKey)
	})

	t.Run("success - same key resolves to own order for each user", func(t *testing.T) {
		defer truncateAll(t)

		idempotencyKey := uuid.New()
		user1, user2 := uuid.New(), uuid.New()
		order1 := newOrder(user1, idempotencyKey, 50000)
		order2 := newOrder(user2, idempotencyKey, 60000)
		require.NoError(t, repo.Create(ctx, order1))
		require.NoError(t, repo.Create(ctx, order2))

		found1, err := repo.GetByUserAndIdempotencyKey(ctx, user1, idempotencyKey)
		require.NoError(t, err)
		assert.Equal(t, order1.ID, found1.ID)

		found2, err := repo.GetByUserAndIdempotencyKey(ctx, user2, idempotencyKey)
		require.NoError(t, err)
		assert.Equal(t, order2.ID, found2.ID)
	})

	t.Run("error - does not return another user's order", func(t *testing.T) {
		defer truncateAll(t)

		idempotencyKey := uuid.New()
		owner, other := uuid.New(), uuid.New()
		require.NoError(t, repo.Create(ctx, newOrder(owner, idempotencyKey, 50000)))

		found, err := repo.GetByUserAndIdempotencyKey(ctx, other, idempotencyKey)
		require.ErrorIs(t, err, domain.ErrNotFound)
		assert.Nil(t, found)
	})

	t.Run("error - key not found", func(t *testing.T) {
		defer truncateAll(t)

		found, err := repo.GetByUserAndIdempotencyKey(ctx, uuid.New(), uuid.New())
		require.ErrorIs(t, err, domain.ErrNotFound)
		assert.Nil(t, found)
	})

	t.Run("success - reads within transaction", func(t *testing.T) {
		defer truncateAll(t)

		userID := uuid.New()
		idempotencyKey := uuid.New()
		order := newOrder(userID, idempotencyKey, 50000)
		require.NoError(t, repo.Create(ctx, order))

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := txmanager.CtxWithTx(ctx, tx)

		found, err := repo.GetByUserAndIdempotencyKey(txCtx, userID, idempotencyKey)
		require.NoError(t, err)
		assert.Equal(t, order.ID, found.ID)

		require.NoError(t, tx.Commit(ctx))
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

		found, err := repo.GetByUserAndIdempotencyKey(ctx, userID, idempotencyKey)
		require.NoError(t, err)
		assert.NotNil(t, found)
	})

	t.Run("concurrent CreateIfAbsent with same idempotency key - exactly one insert", func(t *testing.T) {
		defer truncateAll(t)

		idempotencyKey := uuid.New()
		userID := uuid.New()

		orders := []*domain.Order{
			newOrder(userID, idempotencyKey, 50000),
			newOrder(userID, idempotencyKey, 60000),
			newOrder(userID, idempotencyKey, 70000),
		}

		type result struct {
			inserted bool
			err      error
		}
		results := make(chan result, len(orders))

		for _, order := range orders {
			go func(o *domain.Order) {
				inserted, err := repo.CreateIfAbsent(ctx, o)
				results <- result{inserted: inserted, err: err}
			}(order)
		}

		insertedCount := 0
		for range orders {
			res := <-results
			require.NoError(t, res.err)
			if res.inserted {
				insertedCount++
			}
		}

		assert.Equal(t, 1, insertedCount)

		var count int
		err := testDB.Pool.QueryRow(
			ctx,
			"SELECT COUNT(*) FROM orders WHERE user_id = $1 AND idempotency_key = $2",
			userID, idempotencyKey,
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		found, err := repo.GetByUserAndIdempotencyKey(ctx, userID, idempotencyKey)
		require.NoError(t, err)
		assert.NotNil(t, found)
	})
}

func TestOrders_IdempotencyUniquenessIsScopedToUser(t *testing.T) {
	ctx := context.Background()

	rows, err := testDB.Pool.Query(ctx, `
		SELECT indexdef
		FROM pg_indexes
		WHERE schemaname = current_schema() AND tablename = 'orders'
	`)
	require.NoError(t, err)
	defer rows.Close()

	var defs []string
	for rows.Next() {
		var def string
		require.NoError(t, rows.Scan(&def))
		defs = append(defs, def)
	}
	require.NoError(t, rows.Err())

	hasComposite, hasGlobalUnique := false, false
	for _, def := range defs {
		if !strings.Contains(strings.ToUpper(def), "UNIQUE") {
			continue
		}
		if strings.Contains(def, "(user_id, idempotency_key)") {
			hasComposite = true
		}
		if strings.Contains(def, "(idempotency_key)") {
			hasGlobalUnique = true
		}
	}

	assert.True(t, hasComposite, "ожидался UNIQUE индекс (user_id, idempotency_key); индексы: %v", defs)
	assert.False(t, hasGlobalUnique, "глобальный UNIQUE(idempotency_key) должен быть удалён; индексы: %v", defs)
}
