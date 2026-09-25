package reservationService_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/domain"
	"github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestReservationService_Release(t *testing.T) {
	t.Parallel()

	orderID := uuid.New()
	productID1 := uuid.New().String()
	productID2 := uuid.New().String()

	input := service.ReleaseInput{
		OrderID: orderID,
	}

	t.Run("success - releases all active reservations", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		activeReservations := []*domain.Reservation{
			{
				ID:        uuid.New(),
				OrderID:   orderID,
				ProductID: productID1,
				Quantity:  5,
				Status:    domain.ReservationStatusActive,
				CreatedAt: time.Now(),
			},
			{
				ID:        uuid.New(),
				OrderID:   orderID,
				ProductID: productID2,
				Quantity:  10,
				Status:    domain.ReservationStatusActive,
				CreatedAt: time.Now(),
			},
		}

		s.expectTxSuccess()

		s.locker.EXPECT().
			LockByUUID(gomock.Any(), orderID).
			Return(nil)

		s.reservationRepo.EXPECT().
			GetActiveByOrderID(gomock.Any(), orderID).
			Return(activeReservations, nil)

		increasedProducts := make(map[string]int)
		s.stockRepo.EXPECT().
			IncreaseQuantity(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, productID string, quantity int) error {
				increasedProducts[productID] = quantity
				return nil
			}).Times(2)

		s.reservationRepo.EXPECT().
			ReleaseByOrderID(gomock.Any(), orderID).
			DoAndReturn(func(ctx context.Context, oid uuid.UUID) (int, error) {
				require.Len(t, increasedProducts, 2)
				assert.Equal(t, 5, increasedProducts[productID1])
				assert.Equal(t, 10, increasedProducts[productID2])
				return 2, nil
			})

		err := s.svc.Release(ctx, input)

		require.NoError(t, err)
	})

	t.Run("success - idempotent when no active reservations", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		s.expectTxSuccess()

		s.locker.EXPECT().
			LockByUUID(gomock.Any(), orderID).
			Return(nil)

		s.reservationRepo.EXPECT().
			GetActiveByOrderID(gomock.Any(), orderID).
			Return([]*domain.Reservation{}, nil)

		err := s.svc.Release(ctx, input)

		require.NoError(t, err)
	})

	t.Run("success - releases single reservation", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		activeReservations := []*domain.Reservation{
			{
				ID:        uuid.New(),
				OrderID:   orderID,
				ProductID: productID1,
				Quantity:  15,
				Status:    domain.ReservationStatusActive,
				CreatedAt: time.Now(),
			},
		}

		s.expectTxSuccess()

		s.locker.EXPECT().LockByUUID(gomock.Any(), orderID).Return(nil)
		s.reservationRepo.EXPECT().GetActiveByOrderID(gomock.Any(), orderID).Return(activeReservations, nil)

		s.stockRepo.EXPECT().
			IncreaseQuantity(gomock.Any(), productID1, 15).
			Return(nil)

		s.reservationRepo.EXPECT().
			ReleaseByOrderID(gomock.Any(), orderID).
			Return(1, nil)

		err := s.svc.Release(ctx, input)

		require.NoError(t, err)
	})

	t.Run("success - processes reservations in order", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		activeReservations := []*domain.Reservation{
			{OrderID: orderID, ProductID: "product-a", Quantity: 1},
			{OrderID: orderID, ProductID: "product-b", Quantity: 2},
			{OrderID: orderID, ProductID: "product-c", Quantity: 3},
		}

		s.expectTxSuccess()

		s.locker.EXPECT().LockByUUID(gomock.Any(), orderID).Return(nil)
		s.reservationRepo.EXPECT().GetActiveByOrderID(gomock.Any(), orderID).Return(activeReservations, nil)

		processedOrder := make([]string, 0)
		s.stockRepo.EXPECT().
			IncreaseQuantity(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, productID string, quantity int) error {
				processedOrder = append(processedOrder, productID)
				return nil
			}).Times(3)

		s.reservationRepo.EXPECT().ReleaseByOrderID(gomock.Any(), orderID).Return(3, nil)

		err := s.svc.Release(ctx, input)

		require.NoError(t, err)
		assert.Equal(t, []string{"product-a", "product-b", "product-c"}, processedOrder)
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

		err := s.svc.Release(ctx, input)

		require.Error(t, err)
		var appErr *apperror.AppError
		require.True(t, errors.As(err, &appErr))
		assert.Equal(t, apperror.CodeInternal, appErr.Code())
		assert.Contains(t, appErr.Op(), "locker.LockByUUID")
	})

	t.Run("error - GetActiveByOrderID fails", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		s.expectTxSuccess()

		s.locker.EXPECT().LockByUUID(gomock.Any(), orderID).Return(nil)

		dbErr := errors.New("database error")
		s.reservationRepo.EXPECT().
			GetActiveByOrderID(gomock.Any(), orderID).
			Return(nil, dbErr)

		err := s.svc.Release(ctx, input)

		require.Error(t, err)
		var appErr *apperror.AppError
		require.True(t, errors.As(err, &appErr))
		assert.Equal(t, apperror.CodeInternal, appErr.Code())
		assert.Contains(t, appErr.Op(), "reservationRepo.GetActiveByOrderID")
	})

	t.Run("error - IncreaseQuantity fails on first item", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		activeReservations := []*domain.Reservation{
			{OrderID: orderID, ProductID: productID1, Quantity: 5},
			{OrderID: orderID, ProductID: productID2, Quantity: 10},
		}

		s.expectTxSuccess()

		s.locker.EXPECT().LockByUUID(gomock.Any(), orderID).Return(nil)
		s.reservationRepo.EXPECT().GetActiveByOrderID(gomock.Any(), orderID).Return(activeReservations, nil)

		dbErr := errors.New("database error")
		s.stockRepo.EXPECT().
			IncreaseQuantity(gomock.Any(), productID1, 5).
			Return(dbErr)

		err := s.svc.Release(ctx, input)

		require.Error(t, err)
		var appErr *apperror.AppError
		require.True(t, errors.As(err, &appErr))
		assert.Equal(t, apperror.CodeInternal, appErr.Code())
		assert.Contains(t, appErr.Op(), "stockRepo.IncreaseQuantity")
	})

	t.Run("error - IncreaseQuantity fails on second item", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		activeReservations := []*domain.Reservation{
			{OrderID: orderID, ProductID: productID1, Quantity: 5},
			{OrderID: orderID, ProductID: productID2, Quantity: 10},
		}

		s.expectTxSuccess()

		s.locker.EXPECT().LockByUUID(gomock.Any(), orderID).Return(nil)
		s.reservationRepo.EXPECT().GetActiveByOrderID(gomock.Any(), orderID).Return(activeReservations, nil)

		s.stockRepo.EXPECT().
			IncreaseQuantity(gomock.Any(), productID1, 5).
			Return(nil)

		dbErr := errors.New("database error")
		s.stockRepo.EXPECT().
			IncreaseQuantity(gomock.Any(), productID2, 10).
			Return(dbErr)

		err := s.svc.Release(ctx, input)

		require.Error(t, err)
		var appErr *apperror.AppError
		require.True(t, errors.As(err, &appErr))
		assert.Equal(t, apperror.CodeInternal, appErr.Code())
	})

	t.Run("error - ReleaseByOrderID fails", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		activeReservations := []*domain.Reservation{
			{OrderID: orderID, ProductID: productID1, Quantity: 5},
		}

		s.expectTxSuccess()

		s.locker.EXPECT().LockByUUID(gomock.Any(), orderID).Return(nil)
		s.reservationRepo.EXPECT().GetActiveByOrderID(gomock.Any(), orderID).Return(activeReservations, nil)
		s.stockRepo.EXPECT().IncreaseQuantity(gomock.Any(), productID1, 5).Return(nil)

		dbErr := errors.New("database error")
		s.reservationRepo.EXPECT().
			ReleaseByOrderID(gomock.Any(), orderID).
			Return(0, dbErr)

		err := s.svc.Release(ctx, input)

		require.Error(t, err)
		var appErr *apperror.AppError
		require.True(t, errors.As(err, &appErr))
		assert.Equal(t, apperror.CodeInternal, appErr.Code())
		assert.Contains(t, appErr.Op(), "reservationRepo.ReleaseByOrderID")
	})

	t.Run("error - transaction rollback", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		txErr := errors.New("transaction error")
		s.expectTxFailure(txErr)

		err := s.svc.Release(ctx, input)

		require.Error(t, err)
		assert.Equal(t, txErr, err)
	})

	t.Run("success - handles large number of reservations", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		activeReservations := make([]*domain.Reservation, 100)
		for i := 0; i < 100; i++ {
			activeReservations[i] = &domain.Reservation{
				ID:        uuid.New(),
				OrderID:   orderID,
				ProductID: uuid.New().String(),
				Quantity:  i + 1,
				Status:    domain.ReservationStatusActive,
			}
		}

		s.expectTxSuccess()

		s.locker.EXPECT().LockByUUID(gomock.Any(), orderID).Return(nil)
		s.reservationRepo.EXPECT().GetActiveByOrderID(gomock.Any(), orderID).Return(activeReservations, nil)

		s.stockRepo.EXPECT().
			IncreaseQuantity(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil).
			Times(100)

		s.reservationRepo.EXPECT().
			ReleaseByOrderID(gomock.Any(), orderID).
			Return(100, nil)

		err := s.svc.Release(ctx, input)

		require.NoError(t, err)
	})

	t.Run("success - increases correct quantities for each product", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		activeReservations := []*domain.Reservation{
			{OrderID: orderID, ProductID: "product-1", Quantity: 100},
			{OrderID: orderID, ProductID: "product-2", Quantity: 200},
			{OrderID: orderID, ProductID: "product-3", Quantity: 300},
		}

		s.expectTxSuccess()

		s.locker.EXPECT().LockByUUID(gomock.Any(), orderID).Return(nil)
		s.reservationRepo.EXPECT().GetActiveByOrderID(gomock.Any(), orderID).Return(activeReservations, nil)

		expectedQuantities := map[string]int{
			"product-1": 100,
			"product-2": 200,
			"product-3": 300,
		}

		s.stockRepo.EXPECT().
			IncreaseQuantity(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, productID string, quantity int) error {
				expected, exists := expectedQuantities[productID]
				require.True(t, exists, "unexpected product ID: %s", productID)
				assert.Equal(t, expected, quantity, "wrong quantity for product %s", productID)
				return nil
			}).Times(3)

		s.reservationRepo.EXPECT().ReleaseByOrderID(gomock.Any(), orderID).Return(3, nil)

		err := s.svc.Release(ctx, input)

		require.NoError(t, err)
	})

	t.Run("success - nil active reservations slice", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		s.expectTxSuccess()

		s.locker.EXPECT().LockByUUID(gomock.Any(), orderID).Return(nil)
		s.reservationRepo.EXPECT().
			GetActiveByOrderID(gomock.Any(), orderID).
			Return(nil, nil)

		err := s.svc.Release(ctx, input)

		require.NoError(t, err)
	})

	t.Run("error - partial release on IncreaseQuantity failure rolls back", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		activeReservations := []*domain.Reservation{
			{OrderID: orderID, ProductID: "product-1", Quantity: 10},
			{OrderID: orderID, ProductID: "product-2", Quantity: 20},
			{OrderID: orderID, ProductID: "product-3", Quantity: 30},
		}

		s.txManager.EXPECT().
			WithinTransaction(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
				err := fn(ctx)
				return err
			})

		s.locker.EXPECT().LockByUUID(gomock.Any(), orderID).Return(nil)
		s.reservationRepo.EXPECT().GetActiveByOrderID(gomock.Any(), orderID).Return(activeReservations, nil)

		s.stockRepo.EXPECT().IncreaseQuantity(gomock.Any(), "product-1", 10).Return(nil)
		s.stockRepo.EXPECT().IncreaseQuantity(gomock.Any(), "product-2", 20).Return(nil)

		dbErr := errors.New("database error")
		s.stockRepo.EXPECT().IncreaseQuantity(gomock.Any(), "product-3", 30).Return(dbErr)

		err := s.svc.Release(ctx, input)

		require.Error(t, err)
	})
}
