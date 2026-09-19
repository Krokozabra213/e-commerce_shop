package authservice_test

import (
	"context"
	"testing"

	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/config"
	authservice "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/auth/service"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/auth/service/mocks"
	"go.uber.org/mock/gomock"
)

const hashedPassword = "$2a$10$abcdefghijklmnopqrstuv"

type testSuite struct {
	svc                   *authservice.Service
	userRepo              *mocks.MockUserRepository
	refreshTokenRepo      *mocks.MockRefreshTokenRepository
	emailVerificationRepo *mocks.MockEmailVerificationRepository
	outboxRepo            *mocks.MockOutboxRepository
	jwtManager            *mocks.MockJWTManager
	jwtValidator          *mocks.MockJWTValidator
	jwtKeyStore           *mocks.MockJWTKeyStore
	txManager             *mocks.MockTxManager
	passHasher            *mocks.MockPassHasher
	tokenHasher           *mocks.MockTokenHasher
}

func newTestSuite(t *testing.T) *testSuite {
	t.Helper()

	ctrl := gomock.NewController(t)

	s := &testSuite{
		userRepo:              mocks.NewMockUserRepository(ctrl),
		refreshTokenRepo:      mocks.NewMockRefreshTokenRepository(ctrl),
		emailVerificationRepo: mocks.NewMockEmailVerificationRepository(ctrl),
		outboxRepo:            mocks.NewMockOutboxRepository(ctrl),
		jwtManager:            mocks.NewMockJWTManager(ctrl),
		jwtValidator:          mocks.NewMockJWTValidator(ctrl),
		jwtKeyStore:           mocks.NewMockJWTKeyStore(ctrl),
		txManager:             mocks.NewMockTxManager(ctrl),
		passHasher:            mocks.NewMockPassHasher(ctrl),
		tokenHasher:           mocks.NewMockTokenHasher(ctrl),
	}

	s.svc = authservice.New(
		s.userRepo,
		s.refreshTokenRepo,
		s.emailVerificationRepo,
		s.outboxRepo,
		s.jwtManager,
		s.jwtValidator,
		s.jwtKeyStore,
		s.txManager,
		s.passHasher,
		s.tokenHasher,
		config.AuthJWTConfig{},
		config.EmailVerificationConfig{},
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
