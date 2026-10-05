package authservice_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	jwtmanager "github.com/Krokozabra213/e-commerce_shop/infra/jwt/manager"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
	authservice "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/auth/service"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestService_Refresh(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	oldRefreshToken := "old-refresh-token-string"
	newAccessToken := "new-access-token-string"
	newRefreshToken := "new-refresh-token-string"
	newRefreshExp := time.Now().UTC().Add(7 * 24 * time.Hour)
	oldTokenHash := "hash-of-old-token"
	newTokenHash := "hash-of-new-token"

	validClaims := &jwtmanager.RefreshClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: userID.String(),
		},
	}

	input := authservice.RefreshInput{
		RefreshToken: oldRefreshToken,
	}

	expectPreTxSuccess := func(s *testSuite) {
		s.jwtValidator.EXPECT().
			ValidateRefresh(oldRefreshToken).
			Return(validClaims, nil)

		s.jwtManager.EXPECT().
			GenerateTokens(userID).
			Return(newAccessToken, newRefreshToken, newRefreshExp, nil)

		s.tokenHasher.EXPECT().
			Hash(oldRefreshToken).
			Return(oldTokenHash)

		s.tokenHasher.EXPECT().
			Hash(newRefreshToken).
			Return(newTokenHash)
	}

	activeStoredToken := func() *domain.RefreshToken {
		return &domain.RefreshToken{
			ID:        uuid.New(),
			UserID:    userID,
			TokenHash: oldTokenHash,
			ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
			Revoked:   false,
			CreatedAt: time.Now().UTC().Add(-24 * time.Hour),
		}
	}

	t.Run("success - tokens rotated in correct order", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		expectPreTxSuccess(s)
		s.expectTxSuccess()

		storedToken := activeStoredToken()

		var createdToken *domain.RefreshToken

		gomock.InOrder(
			s.refreshTokenRepo.EXPECT().
				GetByTokenHashForUpdate(gomock.Any(), oldTokenHash).
				Return(storedToken, nil),

			s.refreshTokenRepo.EXPECT().
				Revoke(gomock.Any(), oldTokenHash).
				Return(nil),

			s.refreshTokenRepo.EXPECT().
				Create(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, token *domain.RefreshToken) error {
					createdToken = token

					assert.NotEqual(t, uuid.Nil, token.ID)
					assert.NotEqual(t, storedToken.ID, token.ID, "new token must have a fresh ID")
					assert.Equal(t, userID, token.UserID)
					assert.Equal(t, newTokenHash, token.TokenHash)
					assert.WithinDuration(t, newRefreshExp, token.ExpiresAt, time.Second)
					assert.NotEqual(t, storedToken.ExpiresAt, token.ExpiresAt,
						"new token must not reuse old expiry")
					assert.False(t, token.Revoked)
					assert.False(t, token.CreatedAt.IsZero())
					return nil
				}),
		)

		result, err := s.svc.Refresh(ctx, input)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, createdToken)
		assert.Equal(t, newAccessToken, result.AccessToken)
		assert.Equal(t, newRefreshToken, result.RefreshToken)
	})

	t.Run("error - invalid refresh token", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		s.jwtValidator.EXPECT().
			ValidateRefresh(oldRefreshToken).
			Return(nil, errors.New("token expired"))

		result, err := s.svc.Refresh(ctx, input)

		require.Error(t, err)
		assert.Nil(t, result)

		var bizErr *apperror.AppError
		require.True(t, errors.As(err, &bizErr))
		assert.Equal(t, apperror.CodeUnauthorized, bizErr.Code())
	})

	t.Run("error - invalid subject in claims", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		badClaims := &jwtmanager.RefreshClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				Subject: "not-a-uuid",
			},
		}

		s.jwtValidator.EXPECT().
			ValidateRefresh(oldRefreshToken).
			Return(badClaims, nil)

		result, err := s.svc.Refresh(ctx, input)

		require.Error(t, err)
		assert.Nil(t, result)

		var bizErr *apperror.AppError
		require.True(t, errors.As(err, &bizErr))
		assert.Equal(t, apperror.CodeUnauthorized, bizErr.Code())
	})

	t.Run("error - jwtManager.GenerateTokens fails", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		s.jwtValidator.EXPECT().
			ValidateRefresh(oldRefreshToken).
			Return(validClaims, nil)

		s.jwtManager.EXPECT().
			GenerateTokens(userID).
			Return("", "", time.Time{}, errors.New("key rotation in progress"))

		result, err := s.svc.Refresh(ctx, input)

		require.Error(t, err)
		assert.Nil(t, result)

		var intErr *apperror.AppError
		require.True(t, errors.As(err, &intErr))
		assert.Equal(t, apperror.CodeInternal, intErr.Code())
	})

	t.Run("error - token not found in database", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		expectPreTxSuccess(s)
		s.expectTxSuccess()

		s.refreshTokenRepo.EXPECT().
			GetByTokenHashForUpdate(gomock.Any(), oldTokenHash).
			Return(nil, domain.ErrNotFound)

		result, err := s.svc.Refresh(ctx, input)

		require.Error(t, err)
		assert.Nil(t, result)

		var bizErr *apperror.AppError
		require.True(t, errors.As(err, &bizErr))
		assert.Equal(t, apperror.CodeUnauthorized, bizErr.Code())
	})

	t.Run("error - GetByTokenHashForUpdate returns unexpected error", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		expectPreTxSuccess(s)
		s.expectTxSuccess()

		s.refreshTokenRepo.EXPECT().
			GetByTokenHashForUpdate(gomock.Any(), oldTokenHash).
			Return(nil, errors.New("connection pool exhausted"))

		result, err := s.svc.Refresh(ctx, input)

		require.Error(t, err)
		assert.Nil(t, result)

		var intErr *apperror.AppError
		require.True(t, errors.As(err, &intErr))
		assert.Equal(t, apperror.CodeInternal, intErr.Code())
	})

	t.Run("error - stored token belongs to another user", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		expectPreTxSuccess(s)
		s.expectTxSuccess()

		foreignToken := &domain.RefreshToken{
			ID:        uuid.New(),
			UserID:    uuid.New(),
			TokenHash: oldTokenHash,
			ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
			Revoked:   false,
		}

		s.refreshTokenRepo.EXPECT().
			GetByTokenHashForUpdate(gomock.Any(), oldTokenHash).
			Return(foreignToken, nil)

		result, err := s.svc.Refresh(ctx, input)

		require.Error(t, err)
		assert.Nil(t, result)

		var intErr *apperror.AppError
		require.True(t, errors.As(err, &intErr))
		assert.Equal(t, apperror.CodeInternal, intErr.Code())
	})

	t.Run("error - token already revoked (possible token reuse attack)", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		expectPreTxSuccess(s)
		s.expectTxSuccess()

		revokedToken := &domain.RefreshToken{
			ID:        uuid.New(),
			UserID:    userID,
			TokenHash: oldTokenHash,
			ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
			Revoked:   true,
			CreatedAt: time.Now().UTC().Add(-48 * time.Hour),
		}

		s.refreshTokenRepo.EXPECT().
			GetByTokenHashForUpdate(gomock.Any(), oldTokenHash).
			Return(revokedToken, nil)

		result, err := s.svc.Refresh(ctx, input)

		require.Error(t, err)
		assert.Nil(t, result)

		var bizErr *apperror.AppError
		require.True(t, errors.As(err, &bizErr))
		assert.Equal(t, apperror.CodeUnauthorized, bizErr.Code())
	})

	t.Run("error - stored token is expired", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		expectPreTxSuccess(s)
		s.expectTxSuccess()

		expiredToken := &domain.RefreshToken{
			ID:        uuid.New(),
			UserID:    userID,
			TokenHash: oldTokenHash,
			ExpiresAt: time.Now().UTC().Add(-time.Hour),
			Revoked:   false,
			CreatedAt: time.Now().UTC().Add(-8 * 24 * time.Hour),
		}

		s.refreshTokenRepo.EXPECT().
			GetByTokenHashForUpdate(gomock.Any(), oldTokenHash).
			Return(expiredToken, nil)

		result, err := s.svc.Refresh(ctx, input)

		require.Error(t, err)
		assert.Nil(t, result)

		var bizErr *apperror.AppError
		require.True(t, errors.As(err, &bizErr))
		assert.Equal(t, apperror.CodeUnauthorized, bizErr.Code())
	})

	t.Run("error - Revoke fails", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		expectPreTxSuccess(s)
		s.expectTxSuccess()

		storedToken := activeStoredToken()

		gomock.InOrder(
			s.refreshTokenRepo.EXPECT().
				GetByTokenHashForUpdate(gomock.Any(), oldTokenHash).
				Return(storedToken, nil),

			s.refreshTokenRepo.EXPECT().
				Revoke(gomock.Any(), oldTokenHash).
				Return(errors.New("database write error")),
		)

		result, err := s.svc.Refresh(ctx, input)

		require.Error(t, err)
		assert.Nil(t, result)

		var intErr *apperror.AppError
		require.True(t, errors.As(err, &intErr))
		assert.Equal(t, apperror.CodeInternal, intErr.Code())
	})

	t.Run("error - Create new token fails (tx rollback restores old token)", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		expectPreTxSuccess(s)
		s.expectTxSuccess()

		storedToken := activeStoredToken()

		gomock.InOrder(
			s.refreshTokenRepo.EXPECT().
				GetByTokenHashForUpdate(gomock.Any(), oldTokenHash).
				Return(storedToken, nil),

			s.refreshTokenRepo.EXPECT().
				Revoke(gomock.Any(), oldTokenHash).
				Return(nil),

			s.refreshTokenRepo.EXPECT().
				Create(gomock.Any(), gomock.Any()).
				Return(errors.New("unique violation on token_hash")),
		)

		result, err := s.svc.Refresh(ctx, input)

		require.Error(t, err)
		assert.Nil(t, result)

		var intErr *apperror.AppError
		require.True(t, errors.As(err, &intErr))
		assert.Equal(t, apperror.CodeInternal, intErr.Code())
	})

	t.Run("error - transaction fails", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		expectPreTxSuccess(s)

		txErr := errors.New("deadlock detected")
		s.expectTxFailure(txErr)

		result, err := s.svc.Refresh(ctx, input)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, txErr)
	})
}
