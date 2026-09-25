package reservationService_test

import (
	"context"
	"testing"

	reservationService "github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/service/reservation"
	"github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/service/reservation/mocks"
	"go.uber.org/mock/gomock"
)

type testSuite struct {
	svc             *reservationService.ReservationService
	stockRepo       *mocks.MockStockRepository
	reservationRepo *mocks.MockReservationRepository
	outboxRepo      *mocks.MockOutboxRepository
	locker          *mocks.MockAdvisoryLocker
	txManager       *mocks.MockTxManager
}

func newTestSuite(t *testing.T) *testSuite {
	t.Helper()

	ctrl := gomock.NewController(t)

	s := &testSuite{
		stockRepo:       mocks.NewMockStockRepository(ctrl),
		reservationRepo: mocks.NewMockReservationRepository(ctrl),
		outboxRepo:      mocks.NewMockOutboxRepository(ctrl),
		locker:          mocks.NewMockAdvisoryLocker(ctrl),
		txManager:       mocks.NewMockTxManager(ctrl),
	}

	s.svc = reservationService.NewReservationService(
		s.stockRepo,
		s.reservationRepo,
		s.outboxRepo,
		s.locker,
		s.txManager,
	)

	return s
}

func (s *testSuite) expectTxSuccess() {
	s.txManager.EXPECT().
		WithinTransaction(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})
}

func (s *testSuite) expectTxFailure(txErr error) {
	s.txManager.EXPECT().
		WithinTransaction(gomock.Any(), gomock.Any()).
		Return(txErr)
}

func (s *testSuite) expectTxSuccessTwice() {
	s.txManager.EXPECT().
		WithinTransaction(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).Times(2)
}
