//go:build integration

package reservationRepository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/testutils"
	tx_manager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/domain"
	reservationRepository "github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/repository/postgres/reservation"
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

func newReservation(orderID uuid.UUID, productID string, quantity int) *domain.Reservation {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return &domain.Reservation{
		ID:        uuid.New(),
		OrderID:   orderID,
		ProductID: productID,
		Quantity:  quantity,
		Status:    domain.ReservationStatusActive,
		CreatedAt: now,
	}
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

func createTestStock(t *testing.T, ctx context.Context, stockRepo *stockRepository.PostgresStockRepository, productID string, quantity int) {
	t.Helper()
	stock := newStock(productID, quantity)
	require.NoError(t, stockRepo.Create(ctx, stock))
}

func createTestReservations(t *testing.T, ctx context.Context, repo *reservationRepository.PostgresReservationRepository, reservations []*domain.Reservation) {
	t.Helper()
	require.NoError(t, repo.CreateBatch(ctx, reservations))
}

func TestPostgresReservationRepository_CreateBatch(t *testing.T) {
	ctx := context.Background()
	reservationRepo := reservationRepository.NewPostgresReservationRepository(testDB.Pool)
	stockRepo := stockRepository.NewPostgresStockRepository(testDB.Pool)

	t.Run("success - creates single reservation", func(t *testing.T) {
		defer truncateAll(t)

		orderID := uuid.New()
		productID := uuid.New().String()

		// Создаем stock для продукта
		createTestStock(t, ctx, stockRepo, productID, 100)

		reservation := newReservation(orderID, productID, 10)

		err := reservationRepo.CreateBatch(ctx, []*domain.Reservation{reservation})
		require.NoError(t, err)

		reservations, err := reservationRepo.GetActiveByOrderID(ctx, orderID)
		require.NoError(t, err)
		require.Len(t, reservations, 1)
		assert.Equal(t, reservation.ID, reservations[0].ID)
		assert.Equal(t, reservation.OrderID, reservations[0].OrderID)
		assert.Equal(t, reservation.ProductID, reservations[0].ProductID)
		assert.Equal(t, reservation.Quantity, reservations[0].Quantity)
		assert.Equal(t, domain.ReservationStatusActive, reservations[0].Status)
		assert.WithinDuration(t, reservation.CreatedAt, reservations[0].CreatedAt, time.Second)
		assert.Nil(t, reservations[0].ReleasedAt)
	})

	t.Run("success - creates multiple reservations for same order", func(t *testing.T) {
		defer truncateAll(t)

		orderID := uuid.New()
		productID1 := uuid.New().String()
		productID2 := uuid.New().String()
		productID3 := uuid.New().String()

		// Создаем stocks для продуктов
		createTestStock(t, ctx, stockRepo, productID1, 100)
		createTestStock(t, ctx, stockRepo, productID2, 100)
		createTestStock(t, ctx, stockRepo, productID3, 100)

		reservations := []*domain.Reservation{
			newReservation(orderID, productID1, 5),
			newReservation(orderID, productID2, 10),
			newReservation(orderID, productID3, 15),
		}

		err := reservationRepo.CreateBatch(ctx, reservations)
		require.NoError(t, err)

		found, err := reservationRepo.GetActiveByOrderID(ctx, orderID)
		require.NoError(t, err)
		require.Len(t, found, 3)
	})

	t.Run("success - creates reservations for different orders", func(t *testing.T) {
		defer truncateAll(t)

		orderID1 := uuid.New()
		orderID2 := uuid.New()
		productID := uuid.New().String()

		// Создаем stock для продукта
		createTestStock(t, ctx, stockRepo, productID, 100)

		reservations := []*domain.Reservation{
			newReservation(orderID1, productID, 5),
			newReservation(orderID2, productID, 10),
		}

		err := reservationRepo.CreateBatch(ctx, reservations)
		require.NoError(t, err)

		found1, err := reservationRepo.GetActiveByOrderID(ctx, orderID1)
		require.NoError(t, err)
		require.Len(t, found1, 1)

		found2, err := reservationRepo.GetActiveByOrderID(ctx, orderID2)
		require.NoError(t, err)
		require.Len(t, found2, 1)
	})

	t.Run("success - creates batch within transaction and commits", func(t *testing.T) {
		defer truncateAll(t)

		productID1 := uuid.New().String()
		productID2 := uuid.New().String()

		// Создаем stocks для продуктов
		createTestStock(t, ctx, stockRepo, productID1, 100)
		createTestStock(t, ctx, stockRepo, productID2, 100)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		orderID := uuid.New()
		reservations := []*domain.Reservation{
			newReservation(orderID, productID1, 5),
			newReservation(orderID, productID2, 10),
		}

		err = reservationRepo.CreateBatch(txCtx, reservations)
		require.NoError(t, err)

		require.NoError(t, tx.Commit(ctx))

		found, err := reservationRepo.GetActiveByOrderID(ctx, orderID)
		require.NoError(t, err)
		require.Len(t, found, 2)
	})

	t.Run("success - rollback within transaction does not persist reservations", func(t *testing.T) {
		defer truncateAll(t)

		productID := uuid.New().String()

		// Создаем stock для продукта
		createTestStock(t, ctx, stockRepo, productID, 100)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		orderID := uuid.New()
		reservations := []*domain.Reservation{
			newReservation(orderID, productID, 5),
		}

		require.NoError(t, reservationRepo.CreateBatch(txCtx, reservations))
		require.NoError(t, tx.Rollback(ctx))

		found, err := reservationRepo.GetActiveByOrderID(ctx, orderID)
		require.NoError(t, err)
		require.Empty(t, found)
	})

	t.Run("success - empty slice does nothing", func(t *testing.T) {
		defer truncateAll(t)

		err := reservationRepo.CreateBatch(ctx, []*domain.Reservation{})
		require.NoError(t, err)
	})

	t.Run("success - nil slice does nothing", func(t *testing.T) {
		defer truncateAll(t)

		err := reservationRepo.CreateBatch(ctx, nil)
		require.NoError(t, err)
	})

	t.Run("error - foreign key violation when product does not exist", func(t *testing.T) {
		defer truncateAll(t)

		orderID := uuid.New()
		nonExistentProductID := uuid.New().String()
		reservation := newReservation(orderID, nonExistentProductID, 10)

		err := reservationRepo.CreateBatch(ctx, []*domain.Reservation{reservation})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "foreign key constraint")
	})
}

func TestPostgresReservationRepository_GetActiveByOrderID(t *testing.T) {
	ctx := context.Background()
	reservationRepo := reservationRepository.NewPostgresReservationRepository(testDB.Pool)
	stockRepo := stockRepository.NewPostgresStockRepository(testDB.Pool)

	t.Run("success - gets active reservations for order", func(t *testing.T) {
		defer truncateAll(t)

		orderID := uuid.New()
		productID1 := uuid.New().String()
		productID2 := uuid.New().String()

		// Создаем stocks
		createTestStock(t, ctx, stockRepo, productID1, 100)
		createTestStock(t, ctx, stockRepo, productID2, 100)

		reservations := []*domain.Reservation{
			newReservation(orderID, productID1, 5),
			newReservation(orderID, productID2, 10),
		}

		createTestReservations(t, ctx, reservationRepo, reservations)

		found, err := reservationRepo.GetActiveByOrderID(ctx, orderID)
		require.NoError(t, err)
		require.Len(t, found, 2)

		// Проверяем сортировку по product_id
		if found[0].ProductID > found[1].ProductID {
			found[0], found[1] = found[1], found[0]
		}
		assert.Equal(t, domain.ReservationStatusActive, found[0].Status)
		assert.Equal(t, domain.ReservationStatusActive, found[1].Status)
	})

	t.Run("success - gets active reservations within transaction", func(t *testing.T) {
		defer truncateAll(t)

		productID := uuid.New().String()
		createTestStock(t, ctx, stockRepo, productID, 100)

		orderID := uuid.New()
		reservations := []*domain.Reservation{
			newReservation(orderID, productID, 5),
		}
		createTestReservations(t, ctx, reservationRepo, reservations)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		found, err := reservationRepo.GetActiveByOrderID(txCtx, orderID)
		require.NoError(t, err)
		require.Len(t, found, 1)
	})

	t.Run("success - returns empty slice when no active reservations", func(t *testing.T) {
		defer truncateAll(t)

		orderID := uuid.New()

		found, err := reservationRepo.GetActiveByOrderID(ctx, orderID)
		require.NoError(t, err)
		require.Empty(t, found)
	})

	t.Run("success - does not return released reservations", func(t *testing.T) {
		defer truncateAll(t)

		productID1 := uuid.New().String()
		productID2 := uuid.New().String()
		createTestStock(t, ctx, stockRepo, productID1, 100)
		createTestStock(t, ctx, stockRepo, productID2, 100)

		orderID := uuid.New()
		reservations := []*domain.Reservation{
			newReservation(orderID, productID1, 5),
			newReservation(orderID, productID2, 10),
		}
		createTestReservations(t, ctx, reservationRepo, reservations)

		// Освобождаем резервации
		_, err := reservationRepo.ReleaseByOrderID(ctx, orderID)
		require.NoError(t, err)

		found, err := reservationRepo.GetActiveByOrderID(ctx, orderID)
		require.NoError(t, err)
		require.Empty(t, found)
	})

	t.Run("success - returns only active when both active and released exist", func(t *testing.T) {
		defer truncateAll(t)

		productID := uuid.New().String()
		createTestStock(t, ctx, stockRepo, productID, 100)

		orderID1 := uuid.New()
		orderID2 := uuid.New()

		// Создаем резервации для двух заказов
		reservations1 := []*domain.Reservation{newReservation(orderID1, productID, 5)}
		reservations2 := []*domain.Reservation{newReservation(orderID2, productID, 10)}

		createTestReservations(t, ctx, reservationRepo, reservations1)
		createTestReservations(t, ctx, reservationRepo, reservations2)

		// Освобождаем первый заказ
		_, err := reservationRepo.ReleaseByOrderID(ctx, orderID1)
		require.NoError(t, err)

		// Проверяем что для orderID2 резервация все еще активна
		found, err := reservationRepo.GetActiveByOrderID(ctx, orderID2)
		require.NoError(t, err)
		require.Len(t, found, 1)
		assert.Equal(t, domain.ReservationStatusActive, found[0].Status)
	})

	t.Run("success - orders by product_id", func(t *testing.T) {
		defer truncateAll(t)

		productID1 := "product-a"
		productID2 := "product-b"
		productID3 := "product-c"

		createTestStock(t, ctx, stockRepo, productID1, 100)
		createTestStock(t, ctx, stockRepo, productID2, 100)
		createTestStock(t, ctx, stockRepo, productID3, 100)

		orderID := uuid.New()
		reservations := []*domain.Reservation{
			newReservation(orderID, productID3, 30), // Создаем в произвольном порядке
			newReservation(orderID, productID1, 10),
			newReservation(orderID, productID2, 20),
		}
		createTestReservations(t, ctx, reservationRepo, reservations)

		found, err := reservationRepo.GetActiveByOrderID(ctx, orderID)
		require.NoError(t, err)
		require.Len(t, found, 3)

		// Проверяем сортировку
		assert.Equal(t, productID1, found[0].ProductID)
		assert.Equal(t, productID2, found[1].ProductID)
		assert.Equal(t, productID3, found[2].ProductID)
	})
}

func TestPostgresReservationRepository_ReleaseByOrderID(t *testing.T) {
	ctx := context.Background()
	reservationRepo := reservationRepository.NewPostgresReservationRepository(testDB.Pool)
	stockRepo := stockRepository.NewPostgresStockRepository(testDB.Pool)

	t.Run("success - releases all active reservations for order", func(t *testing.T) {
		defer truncateAll(t)

		productID1 := uuid.New().String()
		productID2 := uuid.New().String()
		createTestStock(t, ctx, stockRepo, productID1, 100)
		createTestStock(t, ctx, stockRepo, productID2, 100)

		orderID := uuid.New()
		reservations := []*domain.Reservation{
			newReservation(orderID, productID1, 5),
			newReservation(orderID, productID2, 10),
		}
		createTestReservations(t, ctx, reservationRepo, reservations)

		count, err := reservationRepo.ReleaseByOrderID(ctx, orderID)
		require.NoError(t, err)
		assert.Equal(t, 2, count)

		// Проверяем что резервации освобождены
		active, err := reservationRepo.GetActiveByOrderID(ctx, orderID)
		require.NoError(t, err)
		assert.Empty(t, active)
	})

	t.Run("success - releases within transaction and commits", func(t *testing.T) {
		defer truncateAll(t)

		productID := uuid.New().String()
		createTestStock(t, ctx, stockRepo, productID, 100)

		orderID := uuid.New()
		reservations := []*domain.Reservation{
			newReservation(orderID, productID, 5),
		}
		createTestReservations(t, ctx, reservationRepo, reservations)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		count, err := reservationRepo.ReleaseByOrderID(txCtx, orderID)
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		require.NoError(t, tx.Commit(ctx))

		active, err := reservationRepo.GetActiveByOrderID(ctx, orderID)
		require.NoError(t, err)
		assert.Empty(t, active)
	})

	t.Run("success - rollback within transaction does not release", func(t *testing.T) {
		defer truncateAll(t)

		productID := uuid.New().String()
		createTestStock(t, ctx, stockRepo, productID, 100)

		orderID := uuid.New()
		reservations := []*domain.Reservation{
			newReservation(orderID, productID, 5),
		}
		createTestReservations(t, ctx, reservationRepo, reservations)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		count, err := reservationRepo.ReleaseByOrderID(txCtx, orderID)
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		require.NoError(t, tx.Rollback(ctx))

		active, err := reservationRepo.GetActiveByOrderID(ctx, orderID)
		require.NoError(t, err)
		require.Len(t, active, 1)
		assert.Equal(t, domain.ReservationStatusActive, active[0].Status)
	})

	t.Run("success - returns 0 when no active reservations", func(t *testing.T) {
		defer truncateAll(t)

		orderID := uuid.New()

		count, err := reservationRepo.ReleaseByOrderID(ctx, orderID)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("success - does not affect other orders", func(t *testing.T) {
		defer truncateAll(t)

		productID := uuid.New().String()
		createTestStock(t, ctx, stockRepo, productID, 100)

		orderID1 := uuid.New()
		orderID2 := uuid.New()

		reservations1 := []*domain.Reservation{newReservation(orderID1, productID, 5)}
		reservations2 := []*domain.Reservation{newReservation(orderID2, productID, 10)}

		createTestReservations(t, ctx, reservationRepo, reservations1)
		createTestReservations(t, ctx, reservationRepo, reservations2)

		count, err := reservationRepo.ReleaseByOrderID(ctx, orderID1)
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		// Проверяем что orderID2 не затронут
		active, err := reservationRepo.GetActiveByOrderID(ctx, orderID2)
		require.NoError(t, err)
		require.Len(t, active, 1)
		assert.Equal(t, domain.ReservationStatusActive, active[0].Status)
	})

	t.Run("success - releasing already released returns 0", func(t *testing.T) {
		defer truncateAll(t)

		productID := uuid.New().String()
		createTestStock(t, ctx, stockRepo, productID, 100)

		orderID := uuid.New()
		reservations := []*domain.Reservation{
			newReservation(orderID, productID, 5),
		}
		createTestReservations(t, ctx, reservationRepo, reservations)

		// Первое освобождение
		count1, err := reservationRepo.ReleaseByOrderID(ctx, orderID)
		require.NoError(t, err)
		assert.Equal(t, 1, count1)

		// Повторное освобождение
		count2, err := reservationRepo.ReleaseByOrderID(ctx, orderID)
		require.NoError(t, err)
		assert.Equal(t, 0, count2)
	})

	t.Run("success - sets released_at timestamp", func(t *testing.T) {
		defer truncateAll(t)

		productID := uuid.New().String()
		createTestStock(t, ctx, stockRepo, productID, 100)

		orderID := uuid.New()
		reservations := []*domain.Reservation{
			newReservation(orderID, productID, 5),
		}
		createTestReservations(t, ctx, reservationRepo, reservations)

		beforeRelease := time.Now().UTC()
		time.Sleep(10 * time.Millisecond)

		_, err := reservationRepo.ReleaseByOrderID(ctx, orderID)
		require.NoError(t, err)

		// Получаем все резервации (не только активные)
		exists, err := reservationRepo.ExistsByOrderID(ctx, orderID)
		require.NoError(t, err)
		assert.True(t, exists)

		// Проверяем что released_at установлен (через прямой запрос)
		var releasedAt *time.Time
		query := `SELECT released_at FROM reservations WHERE order_id = $1`
		err = testDB.Pool.QueryRow(ctx, query, orderID).Scan(&releasedAt)
		require.NoError(t, err)
		require.NotNil(t, releasedAt)
		assert.True(t, releasedAt.After(beforeRelease))
	})
}

func TestPostgresReservationRepository_ExistsByOrderID(t *testing.T) {
	ctx := context.Background()
	reservationRepo := reservationRepository.NewPostgresReservationRepository(testDB.Pool)
	stockRepo := stockRepository.NewPostgresStockRepository(testDB.Pool)

	t.Run("success - returns true when reservations exist", func(t *testing.T) {
		defer truncateAll(t)

		productID := uuid.New().String()
		createTestStock(t, ctx, stockRepo, productID, 100)

		orderID := uuid.New()
		reservations := []*domain.Reservation{
			newReservation(orderID, productID, 5),
		}
		createTestReservations(t, ctx, reservationRepo, reservations)

		exists, err := reservationRepo.ExistsByOrderID(ctx, orderID)
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("success - returns false when no reservations exist", func(t *testing.T) {
		defer truncateAll(t)

		orderID := uuid.New()

		exists, err := reservationRepo.ExistsByOrderID(ctx, orderID)
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("success - returns true even for released reservations", func(t *testing.T) {
		defer truncateAll(t)

		productID := uuid.New().String()
		createTestStock(t, ctx, stockRepo, productID, 100)

		orderID := uuid.New()
		reservations := []*domain.Reservation{
			newReservation(orderID, productID, 5),
		}
		createTestReservations(t, ctx, reservationRepo, reservations)

		_, err := reservationRepo.ReleaseByOrderID(ctx, orderID)
		require.NoError(t, err)

		exists, err := reservationRepo.ExistsByOrderID(ctx, orderID)
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("success - checks within transaction", func(t *testing.T) {
		defer truncateAll(t)

		productID := uuid.New().String()
		createTestStock(t, ctx, stockRepo, productID, 100)

		orderID := uuid.New()
		reservations := []*domain.Reservation{
			newReservation(orderID, productID, 5),
		}
		createTestReservations(t, ctx, reservationRepo, reservations)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		exists, err := reservationRepo.ExistsByOrderID(txCtx, orderID)
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("success - does not see uncommitted transaction data", func(t *testing.T) {
		defer truncateAll(t)

		productID := uuid.New().String()
		createTestStock(t, ctx, stockRepo, productID, 100)

		orderID := uuid.New()
		reservations := []*domain.Reservation{
			newReservation(orderID, productID, 5),
		}

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)

		txCtx := tx_manager.CtxWithTx(ctx, tx)

		// Создаем в транзакции
		require.NoError(t, reservationRepo.CreateBatch(txCtx, reservations))

		// Проверяем вне транзакции - не должно быть видно
		exists, err := reservationRepo.ExistsByOrderID(ctx, orderID)
		require.NoError(t, err)
		assert.False(t, exists)

		// Проверяем в транзакции - должно быть видно
		existsInTx, err := reservationRepo.ExistsByOrderID(txCtx, orderID)
		require.NoError(t, err)
		assert.True(t, existsInTx)

		require.NoError(t, tx.Rollback(ctx))
	})
}
