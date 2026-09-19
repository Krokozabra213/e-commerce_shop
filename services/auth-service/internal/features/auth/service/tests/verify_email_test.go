package authservice_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
	authservice "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/auth/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestService_VerifyEmail(t *testing.T) {
	t.Parallel()

	validToken := "valid-verification-token"
	hashedToken := "hashed-token-value"
	userID := uuid.New()
	now := time.Now()

	input := authservice.VerifyEmailInput{
		Token: validToken,
	}

	t.Run("success - email verified", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		s.expectTxSuccess()

		s.tokenHasher.EXPECT().
			Hash(validToken).
			Return(hashedToken)

		token := &domain.EmailVerificationToken{
			TokenHash: hashedToken,
			UserID:    userID,
			ExpiresAt: now.Add(24 * time.Hour),
			UsedAt:    nil,
			CreatedAt: now.Add(-1 * time.Hour),
		}

		s.emailVerificationRepo.EXPECT().
			GetByTokenHashForUpdate(gomock.Any(), hashedToken).
			Return(token, nil)

		s.emailVerificationRepo.EXPECT().
			MarkAsUsed(gomock.Any(), hashedToken).
			Return(nil)

		s.userRepo.EXPECT().
			ConfirmEmail(gomock.Any(), userID).
			Return(nil)

		err := s.svc.VerifyEmail(ctx, input)

		require.NoError(t, err)
	})

	t.Run("error - token not found", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		s.expectTxSuccess()

		s.tokenHasher.EXPECT().
			Hash(validToken).
			Return(hashedToken)

		s.emailVerificationRepo.EXPECT().
			GetByTokenHashForUpdate(gomock.Any(), hashedToken).
			Return(nil, domain.NotFoundError)

		err := s.svc.VerifyEmail(ctx, input)

		require.Error(t, err)

		var bizErr *apperror.AppError
		require.True(t, errors.As(err, &bizErr))
		assert.Equal(t, apperror.CodeBadRequest, bizErr.Code())
		assert.Contains(t, bizErr.Error(), "Ссылка недействительна")
	})

	t.Run("error - GetByTokenHashForUpdate returns unexpected error", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		s.expectTxSuccess()

		s.tokenHasher.EXPECT().
			Hash(validToken).
			Return(hashedToken)

		s.emailVerificationRepo.EXPECT().
			GetByTokenHashForUpdate(gomock.Any(), hashedToken).
			Return(nil, errors.New("database connection lost"))

		err := s.svc.VerifyEmail(ctx, input)

		require.Error(t, err)

		var intErr *apperror.AppError
		require.True(t, errors.As(err, &intErr))
		assert.Equal(t, apperror.CodeInternal, intErr.Code())
		assert.Contains(t, intErr.Error(), "Токен не найден")
	})

	t.Run("error - token already used", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		s.expectTxSuccess()

		s.tokenHasher.EXPECT().
			Hash(validToken).
			Return(hashedToken)

		usedAt := now.Add(-1 * time.Hour)
		token := &domain.EmailVerificationToken{
			TokenHash: hashedToken,
			UserID:    userID,
			ExpiresAt: now.Add(24 * time.Hour),
			UsedAt:    &usedAt,
			CreatedAt: now.Add(-2 * time.Hour),
		}

		s.emailVerificationRepo.EXPECT().
			GetByTokenHashForUpdate(gomock.Any(), hashedToken).
			Return(token, nil)

		err := s.svc.VerifyEmail(ctx, input)

		require.Error(t, err)

		var bizErr *apperror.AppError
		require.True(t, errors.As(err, &bizErr))
		assert.Equal(t, apperror.CodeBadRequest, bizErr.Code())
		assert.Contains(t, bizErr.Error(), "Ссылка уже была использована")
	})

	t.Run("error - token expired", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		s.expectTxSuccess()

		s.tokenHasher.EXPECT().
			Hash(validToken).
			Return(hashedToken)

		token := &domain.EmailVerificationToken{
			TokenHash: hashedToken,
			UserID:    userID,
			ExpiresAt: now.Add(-1 * time.Hour),
			UsedAt:    nil,
			CreatedAt: now.Add(-25 * time.Hour),
		}

		s.emailVerificationRepo.EXPECT().
			GetByTokenHashForUpdate(gomock.Any(), hashedToken).
			Return(token, nil)

		err := s.svc.VerifyEmail(ctx, input)

		require.Error(t, err)

		var bizErr *apperror.AppError
		require.True(t, errors.As(err, &bizErr))
		assert.Equal(t, apperror.CodeBadRequest, bizErr.Code())
		assert.Contains(t, bizErr.Error(), "Срок действия ссылки истёк")
	})

	t.Run("error - MarkAsUsed fails", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		s.expectTxSuccess()

		s.tokenHasher.EXPECT().
			Hash(validToken).
			Return(hashedToken)

		token := &domain.EmailVerificationToken{
			TokenHash: hashedToken,
			UserID:    userID,
			ExpiresAt: now.Add(24 * time.Hour),
			UsedAt:    nil,
			CreatedAt: now.Add(-1 * time.Hour),
		}

		s.emailVerificationRepo.EXPECT().
			GetByTokenHashForUpdate(gomock.Any(), hashedToken).
			Return(token, nil)

		s.emailVerificationRepo.EXPECT().
			MarkAsUsed(gomock.Any(), hashedToken).
			Return(errors.New("database update failed"))

		err := s.svc.VerifyEmail(ctx, input)

		require.Error(t, err)

		var intErr *apperror.AppError
		require.True(t, errors.As(err, &intErr))
		assert.Equal(t, apperror.CodeInternal, intErr.Code())
		assert.Contains(t, intErr.Error(), "Что-то пошло не так")
	})

	t.Run("error - ConfirmEmail fails", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		s.expectTxSuccess()

		s.tokenHasher.EXPECT().
			Hash(validToken).
			Return(hashedToken)

		token := &domain.EmailVerificationToken{
			TokenHash: hashedToken,
			UserID:    userID,
			ExpiresAt: now.Add(24 * time.Hour),
			UsedAt:    nil,
			CreatedAt: now.Add(-1 * time.Hour),
		}

		s.emailVerificationRepo.EXPECT().
			GetByTokenHashForUpdate(gomock.Any(), hashedToken).
			Return(token, nil)

		s.emailVerificationRepo.EXPECT().
			MarkAsUsed(gomock.Any(), hashedToken).
			Return(nil)

		s.userRepo.EXPECT().
			ConfirmEmail(gomock.Any(), userID).
			Return(errors.New("user not found"))

		err := s.svc.VerifyEmail(ctx, input)

		require.Error(t, err)

		var intErr *apperror.AppError
		require.True(t, errors.As(err, &intErr))
		assert.Equal(t, apperror.CodeInternal, intErr.Code())
		assert.Contains(t, intErr.Error(), "Что-то пошло не так")
	})

	t.Run("error - transaction fails", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		txErr := errors.New("failed to commit transaction")
		s.expectTxFailure(txErr)

		s.tokenHasher.EXPECT().
			Hash(validToken).
			Return(hashedToken)

		err := s.svc.VerifyEmail(ctx, input)

		require.Error(t, err)
		assert.ErrorIs(t, err, txErr)
	})

	t.Run("edge case - token expires exactly now", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		s.expectTxSuccess()

		s.tokenHasher.EXPECT().
			Hash(validToken).
			Return(hashedToken)

		token := &domain.EmailVerificationToken{
			TokenHash: hashedToken,
			UserID:    userID,
			ExpiresAt: time.Now().Add(-1 * time.Millisecond),
			UsedAt:    nil,
			CreatedAt: now.Add(-24 * time.Hour),
		}

		s.emailVerificationRepo.EXPECT().
			GetByTokenHashForUpdate(gomock.Any(), hashedToken).
			Return(token, nil)

		err := s.svc.VerifyEmail(ctx, input)

		require.Error(t, err)

		var bizErr *apperror.AppError
		require.True(t, errors.As(err, &bizErr))
		assert.Equal(t, apperror.CodeBadRequest, bizErr.Code())
		assert.Contains(t, bizErr.Error(), "Срок действия ссылки истёк")
	})

	t.Run("success - token still valid (expires in future)", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		s.expectTxSuccess()

		s.tokenHasher.EXPECT().
			Hash(validToken).
			Return(hashedToken)

		token := &domain.EmailVerificationToken{
			TokenHash: hashedToken,
			UserID:    userID,
			ExpiresAt: time.Now().Add(1 * time.Millisecond),
			UsedAt:    nil,
			CreatedAt: now.Add(-23 * time.Hour),
		}

		s.emailVerificationRepo.EXPECT().
			GetByTokenHashForUpdate(gomock.Any(), hashedToken).
			Return(token, nil)

		s.emailVerificationRepo.EXPECT().
			MarkAsUsed(gomock.Any(), hashedToken).
			Return(nil)

		s.userRepo.EXPECT().
			ConfirmEmail(gomock.Any(), userID).
			Return(nil)

		err := s.svc.VerifyEmail(ctx, input)

		require.NoError(t, err)
	})
}
