//go:build integration

package sagaRepository_tests

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/testutils"
	txmanager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/domain"
	orderRepository "github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/repository/postgres/order"
	sagaRepository "github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/repository/postgres/saga"
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

func newSagaState(orderID, correlationID uuid.UUID) *domain.SagaState {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return &domain.SagaState{
		OrderID:       orderID,
		CorrelationID: correlationID,
		CurrentStep:   domain.SagaStepReservingInventory,
		Status:        domain.SagaStatusInProgress,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func createTestSaga(t *testing.T, ctx context.Context, repo *sagaRepository.PostgresSagaRepository, orderID uuid.UUID) *domain.SagaState {
	t.Helper()
	saga := newSagaState(orderID, uuid.New())
	require.NoError(t, repo.Create(ctx, saga))
	return saga
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

func createTestOrder(t *testing.T, ctx context.Context, repo *orderRepository.PostgresOrderRepository, userID uuid.UUID, totalPrice int64) *domain.Order {
	t.Helper()
	order := newOrder(userID, uuid.New(), totalPrice)
	require.NoError(t, repo.Create(ctx, order))
	return order
}

func TestPostgresSagaRepository_Create(t *testing.T) {
	ctx := context.Background()
	sagaRepo := sagaRepository.NewPostgresSagaRepository(testDB.Pool)
	orderRepo := orderRepository.NewPostgresOrderRepository(testDB.Pool)

	t.Run("success - creates saga without transaction", func(t *testing.T) {
		defer truncateAll(t)

		order := createTestOrder(t, ctx, orderRepo, uuid.New(), 50000)

		correlationID := uuid.New()
		saga := newSagaState(order.ID, correlationID)

		err := sagaRepo.Create(ctx, saga)
		require.NoError(t, err)

		found, err := sagaRepo.GetByOrderID(ctx, order.ID)
		require.NoError(t, err)
		assert.Equal(t, saga.OrderID, found.OrderID)
		assert.Equal(t, saga.CorrelationID, found.CorrelationID)
		assert.Equal(t, saga.CurrentStep, found.CurrentStep)
		assert.Equal(t, saga.Status, found.Status)
		assert.WithinDuration(t, saga.CreatedAt, found.CreatedAt, time.Second)
		assert.WithinDuration(t, saga.UpdatedAt, found.UpdatedAt, time.Second)
	})

	t.Run("success - creates saga with different step", func(t *testing.T) {
		defer truncateAll(t)

		order := createTestOrder(t, ctx, orderRepo, uuid.New(), 50000)

		saga := newSagaState(order.ID, uuid.New())
		saga.CurrentStep = domain.SagaStepChargingPayment

		err := sagaRepo.Create(ctx, saga)
		require.NoError(t, err)

		found, err := sagaRepo.GetByOrderID(ctx, order.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.SagaStepChargingPayment, found.CurrentStep)
	})

	t.Run("success - creates saga within transaction and commits", func(t *testing.T) {
		defer truncateAll(t)

		order := createTestOrder(t, ctx, orderRepo, uuid.New(), 50000)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := txmanager.CtxWithTx(ctx, tx)

		saga := newSagaState(order.ID, uuid.New())
		err = sagaRepo.Create(txCtx, saga)
		require.NoError(t, err)

		require.NoError(t, tx.Commit(ctx))

		found, err := sagaRepo.GetByOrderID(ctx, order.ID)
		require.NoError(t, err)
		assert.Equal(t, saga.OrderID, found.OrderID)
	})

	t.Run("success - rollback within transaction does not persist saga", func(t *testing.T) {
		defer truncateAll(t)

		order := createTestOrder(t, ctx, orderRepo, uuid.New(), 50000)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)

		txCtx := txmanager.CtxWithTx(ctx, tx)

		saga := newSagaState(order.ID, uuid.New())
		require.NoError(t, sagaRepo.Create(txCtx, saga))

		require.NoError(t, tx.Rollback(ctx))

		_, err = sagaRepo.GetByOrderID(ctx, order.ID)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("error - duplicate order_id (primary key)", func(t *testing.T) {
		defer truncateAll(t)

		order := createTestOrder(t, ctx, orderRepo, uuid.New(), 50000)

		saga1 := newSagaState(order.ID, uuid.New())
		saga2 := newSagaState(order.ID, uuid.New())

		require.NoError(t, sagaRepo.Create(ctx, saga1))

		err := sagaRepo.Create(ctx, saga2)
		require.ErrorIs(t, err, domain.ErrAlreadyExists)
	})

	t.Run("error - duplicate correlation_id", func(t *testing.T) {
		defer truncateAll(t)

		order1 := createTestOrder(t, ctx, orderRepo, uuid.New(), 50000)
		order2 := createTestOrder(t, ctx, orderRepo, uuid.New(), 60000)

		correlationID := uuid.New()

		saga1 := newSagaState(order1.ID, correlationID)
		saga2 := newSagaState(order2.ID, correlationID)

		require.NoError(t, sagaRepo.Create(ctx, saga1))

		err := sagaRepo.Create(ctx, saga2)
		require.ErrorIs(t, err, domain.ErrAlreadyExists)
	})

	t.Run("error - foreign key violation for non-existent order", func(t *testing.T) {
		defer truncateAll(t)

		nonExistentOrderID := uuid.New()
		saga := newSagaState(nonExistentOrderID, uuid.New())

		err := sagaRepo.Create(ctx, saga)
		require.Error(t, err)
	})

	t.Run("success - different saga statuses", func(t *testing.T) {
		defer truncateAll(t)

		testCases := []struct {
			name   string
			status domain.SagaStatus
		}{
			{"in_progress", domain.SagaStatusInProgress},
			{"completed", domain.SagaStatusCompleted},
			{"failed", domain.SagaStatusFailed},
			{"compensating", domain.SagaStatusCompensating},
			{"compensated", domain.SagaStatusCompensated},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				order := createTestOrder(t, ctx, orderRepo, uuid.New(), 50000)

				saga := newSagaState(order.ID, uuid.New())
				saga.Status = tc.status

				err := sagaRepo.Create(ctx, saga)
				require.NoError(t, err)

				found, err := sagaRepo.GetByOrderID(ctx, order.ID)
				require.NoError(t, err)
				assert.Equal(t, tc.status, found.Status)
			})
		}
	})

	t.Run("success - different saga steps", func(t *testing.T) {
		defer truncateAll(t)

		testCases := []struct {
			name string
			step domain.SagaStep
		}{
			{"reserving_inventory", domain.SagaStepReservingInventory},
			{"charging_payment", domain.SagaStepChargingPayment},
			{"compensating_payment", domain.SagaStepCompensatingPayment},
			{"compensating_inventory", domain.SagaStepCompensatingInventory},
			{"completed", domain.SagaStepCompleted},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				order := createTestOrder(t, ctx, orderRepo, uuid.New(), 50000)

				saga := newSagaState(order.ID, uuid.New())
				saga.CurrentStep = tc.step

				err := sagaRepo.Create(ctx, saga)
				require.NoError(t, err)

				found, err := sagaRepo.GetByOrderID(ctx, order.ID)
				require.NoError(t, err)
				assert.Equal(t, tc.step, found.CurrentStep)
			})
		}
	})
}

func TestPostgresSagaRepository_GetByOrderID(t *testing.T) {
	ctx := context.Background()
	sagaRepo := sagaRepository.NewPostgresSagaRepository(testDB.Pool)
	orderRepo := orderRepository.NewPostgresOrderRepository(testDB.Pool)

	t.Run("success - finds existing saga", func(t *testing.T) {
		defer truncateAll(t)

		order := createTestOrder(t, ctx, orderRepo, uuid.New(), 50000)
		saga := createTestSaga(t, ctx, sagaRepo, order.ID)

		found, err := sagaRepo.GetByOrderID(ctx, order.ID)
		require.NoError(t, err)
		assert.Equal(t, saga.OrderID, found.OrderID)
		assert.Equal(t, saga.CorrelationID, found.CorrelationID)
	})

	t.Run("error - saga not found", func(t *testing.T) {
		defer truncateAll(t)

		nonExistentOrderID := uuid.New()

		_, err := sagaRepo.GetByOrderID(ctx, nonExistentOrderID)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("success - reads within transaction", func(t *testing.T) {
		defer truncateAll(t)

		order := createTestOrder(t, ctx, orderRepo, uuid.New(), 50000)
		saga := createTestSaga(t, ctx, sagaRepo, order.ID)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := txmanager.CtxWithTx(ctx, tx)

		found, err := sagaRepo.GetByOrderID(txCtx, order.ID)
		require.NoError(t, err)
		assert.Equal(t, saga.OrderID, found.OrderID)

		require.NoError(t, tx.Commit(ctx))
	})
}

func TestPostgresSagaRepository_Update(t *testing.T) {
	ctx := context.Background()
	sagaRepo := sagaRepository.NewPostgresSagaRepository(testDB.Pool)
	orderRepo := orderRepository.NewPostgresOrderRepository(testDB.Pool)

	t.Run("success - updates saga step and status", func(t *testing.T) {
		defer truncateAll(t)

		order := createTestOrder(t, ctx, orderRepo, uuid.New(), 50000)
		saga := createTestSaga(t, ctx, sagaRepo, order.ID)

		assert.Equal(t, domain.SagaStepReservingInventory, saga.CurrentStep)
		assert.Equal(t, domain.SagaStatusInProgress, saga.Status)

		saga.CurrentStep = domain.SagaStepChargingPayment
		saga.Status = domain.SagaStatusInProgress
		saga.UpdatedAt = time.Now().UTC().Truncate(time.Microsecond)

		err := sagaRepo.Update(ctx, saga)
		require.NoError(t, err)

		updated, err := sagaRepo.GetByOrderID(ctx, order.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.SagaStepChargingPayment, updated.CurrentStep)
		assert.Equal(t, domain.SagaStatusInProgress, updated.Status)
		assert.True(t, updated.UpdatedAt.After(saga.CreatedAt))
	})

	t.Run("success - updates to completed state", func(t *testing.T) {
		defer truncateAll(t)

		order := createTestOrder(t, ctx, orderRepo, uuid.New(), 50000)
		saga := createTestSaga(t, ctx, sagaRepo, order.ID)

		saga.CurrentStep = domain.SagaStepCompleted
		saga.Status = domain.SagaStatusCompleted
		saga.UpdatedAt = time.Now().UTC().Truncate(time.Microsecond)

		err := sagaRepo.Update(ctx, saga)
		require.NoError(t, err)

		updated, err := sagaRepo.GetByOrderID(ctx, order.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.SagaStepCompleted, updated.CurrentStep)
		assert.Equal(t, domain.SagaStatusCompleted, updated.Status)
	})

	t.Run("success - updates to compensating state", func(t *testing.T) {
		defer truncateAll(t)

		order := createTestOrder(t, ctx, orderRepo, uuid.New(), 50000)
		saga := createTestSaga(t, ctx, sagaRepo, order.ID)

		saga.CurrentStep = domain.SagaStepCompensatingInventory
		saga.Status = domain.SagaStatusCompensating
		saga.UpdatedAt = time.Now().UTC().Truncate(time.Microsecond)

		err := sagaRepo.Update(ctx, saga)
		require.NoError(t, err)

		updated, err := sagaRepo.GetByOrderID(ctx, order.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.SagaStepCompensatingInventory, updated.CurrentStep)
		assert.Equal(t, domain.SagaStatusCompensating, updated.Status)

		saga.CurrentStep = domain.SagaStepCompleted
		saga.Status = domain.SagaStatusCompensated
		saga.UpdatedAt = time.Now().UTC().Truncate(time.Microsecond)

		err = sagaRepo.Update(ctx, saga)
		require.NoError(t, err)

		final, err := sagaRepo.GetByOrderID(ctx, order.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.SagaStepCompleted, final.CurrentStep)
		assert.Equal(t, domain.SagaStatusCompensated, final.Status)
	})

	t.Run("success - updates within transaction and commits", func(t *testing.T) {
		defer truncateAll(t)

		order := createTestOrder(t, ctx, orderRepo, uuid.New(), 50000)
		saga := createTestSaga(t, ctx, sagaRepo, order.ID)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := txmanager.CtxWithTx(ctx, tx)

		saga.CurrentStep = domain.SagaStepCompleted
		saga.Status = domain.SagaStatusCompleted
		saga.UpdatedAt = time.Now().UTC().Truncate(time.Microsecond)

		err = sagaRepo.Update(txCtx, saga)
		require.NoError(t, err)

		require.NoError(t, tx.Commit(ctx))

		updated, err := sagaRepo.GetByOrderID(ctx, order.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.SagaStepCompleted, updated.CurrentStep)
		assert.Equal(t, domain.SagaStatusCompleted, updated.Status)
	})

	t.Run("success - rollback does not update saga", func(t *testing.T) {
		defer truncateAll(t)

		order := createTestOrder(t, ctx, orderRepo, uuid.New(), 50000)
		saga := createTestSaga(t, ctx, sagaRepo, order.ID)

		originalStep := saga.CurrentStep
		originalStatus := saga.Status

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)

		txCtx := txmanager.CtxWithTx(ctx, tx)

		saga.CurrentStep = domain.SagaStepCompleted
		saga.Status = domain.SagaStatusFailed
		saga.UpdatedAt = time.Now().UTC().Truncate(time.Microsecond)

		require.NoError(t, sagaRepo.Update(txCtx, saga))
		require.NoError(t, tx.Rollback(ctx))

		unchanged, err := sagaRepo.GetByOrderID(ctx, order.ID)
		require.NoError(t, err)
		assert.Equal(t, originalStep, unchanged.CurrentStep)
		assert.Equal(t, originalStatus, unchanged.Status)
	})

	t.Run("error - update non-existent saga", func(t *testing.T) {
		defer truncateAll(t)

		nonExistentOrderID := uuid.New()
		saga := newSagaState(nonExistentOrderID, uuid.New())
		saga.CurrentStep = domain.SagaStepCompleted
		saga.Status = domain.SagaStatusCompleted

		err := sagaRepo.Update(ctx, saga)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("success - multiple sequential updates (happy path)", func(t *testing.T) {
		defer truncateAll(t)

		order := createTestOrder(t, ctx, orderRepo, uuid.New(), 50000)
		saga := createTestSaga(t, ctx, sagaRepo, order.ID)

		saga.CurrentStep = domain.SagaStepChargingPayment
		saga.UpdatedAt = time.Now().UTC().Truncate(time.Microsecond)
		require.NoError(t, sagaRepo.Update(ctx, saga))

		saga.CurrentStep = domain.SagaStepCompleted
		saga.Status = domain.SagaStatusCompleted
		saga.UpdatedAt = time.Now().UTC().Truncate(time.Microsecond)
		require.NoError(t, sagaRepo.Update(ctx, saga))

		final, err := sagaRepo.GetByOrderID(ctx, order.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.SagaStepCompleted, final.CurrentStep)
		assert.Equal(t, domain.SagaStatusCompleted, final.Status)
	})

	t.Run("success - multiple sequential updates (compensation path)", func(t *testing.T) {
		defer truncateAll(t)

		order := createTestOrder(t, ctx, orderRepo, uuid.New(), 50000)
		saga := createTestSaga(t, ctx, sagaRepo, order.ID)

		saga.CurrentStep = domain.SagaStepChargingPayment
		saga.UpdatedAt = time.Now().UTC().Truncate(time.Microsecond)
		require.NoError(t, sagaRepo.Update(ctx, saga))

		saga.CurrentStep = domain.SagaStepCompensatingInventory
		saga.Status = domain.SagaStatusCompensating
		saga.UpdatedAt = time.Now().UTC().Truncate(time.Microsecond)
		require.NoError(t, sagaRepo.Update(ctx, saga))

		saga.CurrentStep = domain.SagaStepCompleted
		saga.Status = domain.SagaStatusCompensated
		saga.UpdatedAt = time.Now().UTC().Truncate(time.Microsecond)
		require.NoError(t, sagaRepo.Update(ctx, saga))

		final, err := sagaRepo.GetByOrderID(ctx, order.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.SagaStepCompleted, final.CurrentStep)
		assert.Equal(t, domain.SagaStatusCompensated, final.Status)
	})
}

func TestPostgresSagaRepository_TransactionalConsistency(t *testing.T) {
	ctx := context.Background()
	sagaRepo := sagaRepository.NewPostgresSagaRepository(testDB.Pool)
	orderRepo := orderRepository.NewPostgresOrderRepository(testDB.Pool)

	t.Run("success - order and saga created atomically", func(t *testing.T) {
		defer truncateAll(t)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := txmanager.CtxWithTx(ctx, tx)

		order := newOrder(uuid.New(), uuid.New(), 50000)
		require.NoError(t, orderRepo.Create(txCtx, order))

		saga := newSagaState(order.ID, uuid.New())
		require.NoError(t, sagaRepo.Create(txCtx, saga))

		require.NoError(t, tx.Commit(ctx))

		_, err = orderRepo.GetByID(ctx, order.ID)
		require.NoError(t, err)

		_, err = sagaRepo.GetByOrderID(ctx, order.ID)
		require.NoError(t, err)
	})

	t.Run("success - rollback prevents both order and saga", func(t *testing.T) {
		defer truncateAll(t)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)

		txCtx := txmanager.CtxWithTx(ctx, tx)

		order := newOrder(uuid.New(), uuid.New(), 50000)
		require.NoError(t, orderRepo.Create(txCtx, order))

		saga := newSagaState(order.ID, uuid.New())
		require.NoError(t, sagaRepo.Create(txCtx, saga))

		require.NoError(t, tx.Rollback(ctx))

		_, err = orderRepo.GetByID(ctx, order.ID)
		require.ErrorIs(t, err, domain.ErrNotFound)

		_, err = sagaRepo.GetByOrderID(ctx, order.ID)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("success - order status and saga step updated atomically", func(t *testing.T) {
		defer truncateAll(t)

		order := createTestOrder(t, ctx, orderRepo, uuid.New(), 50000)
		saga := createTestSaga(t, ctx, sagaRepo, order.ID)

		tx, err := testDB.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		txCtx := txmanager.CtxWithTx(ctx, tx)

		require.NoError(t, orderRepo.UpdateStatus(txCtx, order.ID, domain.OrderStatusReserved))

		saga.CurrentStep = domain.SagaStepChargingPayment
		saga.UpdatedAt = time.Now().UTC().Truncate(time.Microsecond)
		require.NoError(t, sagaRepo.Update(txCtx, saga))

		require.NoError(t, tx.Commit(ctx))

		updatedOrder, err := orderRepo.GetByID(ctx, order.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.OrderStatusReserved, updatedOrder.Status)

		updatedSaga, err := sagaRepo.GetByOrderID(ctx, order.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.SagaStepChargingPayment, updatedSaga.CurrentStep)
	})
}
