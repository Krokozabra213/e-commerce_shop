package reservationService_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/domain"
	"github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestReservationService_Reserve(t *testing.T) {
	t.Parallel()

	orderID := uuid.New()
	correlationID := uuid.New()
	productID1 := uuid.New().String()
	productID2 := uuid.New().String()

	input := service.ReserveInput{
		OrderID:       orderID,
		CorrelationID: correlationID,
		Items: []service.ProductItem{
			{ProductID: productID1, Quantity: 5},
			{ProductID: productID2, Quantity: 10},
		},
	}

	t.Run("success - reserves stock for new order", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		s.expectTxSuccess()

		s.locker.EXPECT().
			LockByUUID(gomock.Any(), orderID).
			Return(nil)

		s.reservationRepo.EXPECT().
			ExistsByOrderID(gomock.Any(), orderID).
			Return(false, nil)

		gomock.InOrder(
			s.stockRepo.EXPECT().
				DecreaseQuantity(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, productID string, quantity int) error {
					return nil
				}).Times(2),
		)

		var createdReservations []*domain.Reservation

		s.reservationRepo.EXPECT().
			CreateBatch(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, reservations []*domain.Reservation) error {
				createdReservations = reservations
				assert.Len(t, reservations, 2)
				return nil
			})

		s.outboxRepo.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, event *domain.OutboxEvent) error {
				require.NotEmpty(t, createdReservations)
				assert.Equal(t, "inventory.reserved", event.EventType)
				return nil
			})

		err := s.svc.Reserve(ctx, input)

		require.NoError(t, err)
	})

	t.Run("error - insufficient stock triggers failure event", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		// Ожидаем ОДНУ транзакцию для резервации
		s.expectTxSuccess()

		s.locker.EXPECT().
			LockByUUID(gomock.Any(), orderID).
			Return(nil)

		s.reservationRepo.EXPECT().
			ExistsByOrderID(gomock.Any(), orderID).
			Return(false, nil)

		s.stockRepo.EXPECT().
			DecreaseQuantity(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(domain.ErrInsufficientStock)

		// Ожидаем публикацию события о неудаче ВНЕ транзакции
		s.outboxRepo.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, event *domain.OutboxEvent) error {
				assert.Equal(t, "inventory.reservation-failed", event.EventType)

				// Проверяем payload
				payloadStr := string(event.Payload)
				assert.Contains(t, payloadStr, "INSUFFICIENT_STOCK")
				return nil
			})

		err := s.svc.Reserve(ctx, input)

		require.NoError(t, err) // Метод возвращает nil при успешной публикации failure
	})

	t.Run("error - insufficient stock and failure event already exists", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		s.expectTxSuccess()

		s.locker.EXPECT().LockByUUID(gomock.Any(), orderID).Return(nil)
		s.reservationRepo.EXPECT().ExistsByOrderID(gomock.Any(), orderID).Return(false, nil)
		s.stockRepo.EXPECT().DecreaseQuantity(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(domain.ErrInsufficientStock)

		// Публикация failure события ВНЕ транзакции
		s.outboxRepo.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			Return(domain.ErrAlreadyExists)

		err := s.svc.Reserve(ctx, input)

		require.NoError(t, err) // AlreadyExists при failure event тоже считается успехом
	})

	t.Run("error - lock acquisition fails", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		s.expectTxSuccess()

		lockErr := errors.New("lock error")
		s.locker.EXPECT().
			LockByUUID(gomock.Any(), orderID).
			Return(lockErr)

		// Ожидаем публикацию failure события ВНЕ транзакции
		s.outboxRepo.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, event *domain.OutboxEvent) error {
				assert.Equal(t, "inventory.reservation-failed", event.EventType)
				payloadStr := string(event.Payload)
				assert.Contains(t, payloadStr, "INTERNAL")
				return nil
			})

		err := s.svc.Reserve(ctx, input)

		require.NoError(t, err)
	})

	t.Run("error - ExistsByOrderID fails", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		s.expectTxSuccess()

		s.locker.EXPECT().LockByUUID(gomock.Any(), orderID).Return(nil)

		dbErr := errors.New("database error")
		s.reservationRepo.EXPECT().
			ExistsByOrderID(gomock.Any(), orderID).
			Return(false, dbErr)

		// Ожидаем публикацию failure события
		s.outboxRepo.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			Return(nil)

		err := s.svc.Reserve(ctx, input)

		require.NoError(t, err)
	})

	t.Run("error - DecreaseQuantity fails with internal error", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		s.expectTxSuccess()

		s.locker.EXPECT().LockByUUID(gomock.Any(), orderID).Return(nil)
		s.reservationRepo.EXPECT().ExistsByOrderID(gomock.Any(), orderID).Return(false, nil)

		dbErr := errors.New("database error")
		s.stockRepo.EXPECT().
			DecreaseQuantity(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(dbErr)

		// Ожидаем публикацию failure события
		s.outboxRepo.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, event *domain.OutboxEvent) error {
				payloadStr := string(event.Payload)
				assert.Contains(t, payloadStr, "INTERNAL")
				return nil
			})

		err := s.svc.Reserve(ctx, input)

		require.NoError(t, err)
	})

	t.Run("error - CreateBatch fails", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		s.expectTxSuccess()

		s.locker.EXPECT().LockByUUID(gomock.Any(), orderID).Return(nil)
		s.reservationRepo.EXPECT().ExistsByOrderID(gomock.Any(), orderID).Return(false, nil)
		s.stockRepo.EXPECT().DecreaseQuantity(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)

		dbErr := errors.New("database error")
		s.reservationRepo.EXPECT().
			CreateBatch(gomock.Any(), gomock.Any()).
			Return(dbErr)

		// Ожидаем публикацию failure события
		s.outboxRepo.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			Return(nil)

		err := s.svc.Reserve(ctx, input)

		require.NoError(t, err)
	})

	t.Run("error - outbox Create fails for success event", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		s.expectTxSuccess()

		s.locker.EXPECT().LockByUUID(gomock.Any(), orderID).Return(nil)
		s.reservationRepo.EXPECT().ExistsByOrderID(gomock.Any(), orderID).Return(false, nil)
		s.stockRepo.EXPECT().DecreaseQuantity(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)
		s.reservationRepo.EXPECT().CreateBatch(gomock.Any(), gomock.Any()).Return(nil)

		outboxErr := errors.New("outbox error")

		// Первый вызов - success событие (в транзакции) - падает
		// Второй вызов - failure событие (вне транзакции) - успех
		gomock.InOrder(
			s.outboxRepo.EXPECT().
				Create(gomock.Any(), gomock.Any()).
				Return(outboxErr),
			s.outboxRepo.EXPECT().
				Create(gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, event *domain.OutboxEvent) error {
					assert.Equal(t, "inventory.reservation-failed", event.EventType)
					return nil
				}),
		)

		err := s.svc.Reserve(ctx, input)

		require.NoError(t, err)
	})

	t.Run("error - outbox Create fails for failure event", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		s.expectTxSuccess()

		s.locker.EXPECT().LockByUUID(gomock.Any(), orderID).Return(nil)
		s.reservationRepo.EXPECT().ExistsByOrderID(gomock.Any(), orderID).Return(false, nil)
		s.stockRepo.EXPECT().DecreaseQuantity(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(domain.ErrInsufficientStock)

		outboxErr := errors.New("outbox error")
		s.outboxRepo.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			Return(outboxErr)

		err := s.svc.Reserve(ctx, input)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "reserve failed and outbox write failed")
	})

	t.Run("error - transaction rollback on DecreaseQuantity failure", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		s.expectTxSuccess()

		s.locker.EXPECT().LockByUUID(gomock.Any(), orderID).Return(nil)
		s.reservationRepo.EXPECT().ExistsByOrderID(gomock.Any(), orderID).Return(false, nil)

		dbErr := errors.New("database error")
		s.stockRepo.EXPECT().DecreaseQuantity(gomock.Any(), gomock.Any(), gomock.Any()).Return(dbErr)

		// Ожидаем публикацию failure события вне транзакции
		s.outboxRepo.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			Return(nil)

		err := s.svc.Reserve(ctx, input)

		require.NoError(t, err)
	})

	// Остальные тесты без изменений...
	t.Run("success - idempotent when reservation already exists", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		s.expectTxSuccess()

		s.locker.EXPECT().
			LockByUUID(gomock.Any(), orderID).
			Return(nil)

		s.reservationRepo.EXPECT().
			ExistsByOrderID(gomock.Any(), orderID).
			Return(true, nil)

		err := s.svc.Reserve(ctx, input)

		require.NoError(t, err)
	})

	t.Run("success - sorts items by product_id", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		unsortedInput := service.ReserveInput{
			OrderID:       orderID,
			CorrelationID: correlationID,
			Items: []service.ProductItem{
				{ProductID: "product-z", Quantity: 30},
				{ProductID: "product-a", Quantity: 10},
				{ProductID: "product-m", Quantity: 20},
			},
		}

		s.expectTxSuccess()

		s.locker.EXPECT().LockByUUID(gomock.Any(), orderID).Return(nil)
		s.reservationRepo.EXPECT().ExistsByOrderID(gomock.Any(), orderID).Return(false, nil)

		callOrder := make([]string, 0, 3)

		s.stockRepo.EXPECT().
			DecreaseQuantity(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, productID string, quantity int) error {
				callOrder = append(callOrder, productID)
				return nil
			}).Times(3)

		s.reservationRepo.EXPECT().CreateBatch(gomock.Any(), gomock.Any()).Return(nil)
		s.outboxRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

		err := s.svc.Reserve(ctx, unsortedInput)

		require.NoError(t, err)
		assert.Equal(t, []string{"product-a", "product-m", "product-z"}, callOrder)
	})

	t.Run("success - handles single item reservation", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		singleItemInput := service.ReserveInput{
			OrderID:       uuid.New(),
			CorrelationID: uuid.New(),
			Items: []service.ProductItem{
				{ProductID: productID1, Quantity: 5},
			},
		}

		s.expectTxSuccess()

		s.locker.EXPECT().LockByUUID(gomock.Any(), singleItemInput.OrderID).Return(nil)
		s.reservationRepo.EXPECT().ExistsByOrderID(gomock.Any(), singleItemInput.OrderID).Return(false, nil)
		s.stockRepo.EXPECT().DecreaseQuantity(gomock.Any(), productID1, 5).Return(nil)

		s.reservationRepo.EXPECT().
			CreateBatch(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, reservations []*domain.Reservation) error {
				assert.Len(t, reservations, 1)
				assert.Equal(t, productID1, reservations[0].ProductID)
				assert.Equal(t, 5, reservations[0].Quantity)
				return nil
			})

		s.outboxRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

		err := s.svc.Reserve(ctx, singleItemInput)

		require.NoError(t, err)
	})

	t.Run("success - processes items in sorted order to prevent deadlocks", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		input1 := service.ReserveInput{
			OrderID:       uuid.New(),
			CorrelationID: uuid.New(),
			Items: []service.ProductItem{
				{ProductID: "product-z", Quantity: 1},
				{ProductID: "product-a", Quantity: 1},
			},
		}

		s.expectTxSuccess()

		s.locker.EXPECT().LockByUUID(gomock.Any(), input1.OrderID).Return(nil)
		s.reservationRepo.EXPECT().ExistsByOrderID(gomock.Any(), input1.OrderID).Return(false, nil)

		processedOrder := make([]string, 0)
		s.stockRepo.EXPECT().
			DecreaseQuantity(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, productID string, quantity int) error {
				processedOrder = append(processedOrder, productID)
				return nil
			}).Times(2)

		s.reservationRepo.EXPECT().CreateBatch(gomock.Any(), gomock.Any()).Return(nil)
		s.outboxRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

		err := s.svc.Reserve(ctx, input1)

		require.NoError(t, err)
		assert.Equal(t, []string{"product-a", "product-z"}, processedOrder)
	})
}
