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

func TestService_Register(t *testing.T) {
	t.Parallel()

	input := authservice.RegisterInput{
		Email:    "newuser@example.com",
		Password: "StrongP@ssw0rd!",
	}

	t.Run("success - new user registered", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		s.expectTxSuccess()

		s.userRepo.EXPECT().
			GetByEmail(gomock.Any(), input.Email).
			Return(nil, domain.NotFoundError)

		s.passHasher.EXPECT().
			Hash(input.Password).
			Return(hashedPassword, nil)

		var createdUser *domain.User

		s.userRepo.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, user *domain.User) error {
				createdUser = user

				assert.NotEqual(t, uuid.Nil, user.ID)
				assert.Equal(t, input.Email, user.Email)
				assert.NotEqual(t, input.Password, derefOr(user.PasswordHash))
				if assert.NotNil(t, user.PasswordHash) {
					assert.Equal(t, hashedPassword, *user.PasswordHash)
				}
				assert.False(t, user.CreatedAt.IsZero())
				assert.False(t, user.UpdatedAt.IsZero())
				assert.True(t, user.CreatedAt.Equal(user.UpdatedAt))
				return nil
			})

		s.outboxRepo.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, event *domain.OutboxEvent) error {
				require.NotNil(t, createdUser, "userRepo.Create must be called before outbox.Create")

				assert.NotEqual(t, uuid.Nil, event.ID)
				assert.Equal(t, "user", event.AggregateType)
				assert.Equal(t, "user.created", event.EventType)
				assert.Equal(t, createdUser.ID, event.AggregateID)
				assert.False(t, event.CreatedAt.IsZero())
				assert.True(t, event.CreatedAt.Equal(createdUser.CreatedAt))

				require.NotNil(t, event.Payload)
				assert.Equal(t, createdUser.ID.String(), event.Payload["id"])
				assert.Equal(t, input.Email, event.Payload["email"])
				assert.Equal(t, []string{"ROLE_USER"}, event.Payload["roles"])

				createdAt, ok := event.Payload["created_at"].(string)
				if assert.True(t, ok, "created_at must be a string") {
					assert.Equal(t, createdUser.CreatedAt.Format(time.RFC3339), createdAt)
				}
				return nil
			})

		result, err := s.svc.Register(ctx, input)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, createdUser)
		assert.Equal(t, createdUser.ID, result.UserID)
	})

	t.Run("error - user already exists", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		s.expectTxSuccess()

		existingUser := &domain.User{Email: input.Email}
		s.userRepo.EXPECT().
			GetByEmail(gomock.Any(), input.Email).
			Return(existingUser, nil)

		result, err := s.svc.Register(ctx, input)

		require.Error(t, err)
		assert.Nil(t, result)

		var bizErr *apperror.AppError
		require.True(t, errors.As(err, &bizErr))
		assert.Equal(t, apperror.CodeAlreadyExists, bizErr.Code())
	})

	t.Run("error - GetByEmail returns unexpected error", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		s.expectTxSuccess()

		s.userRepo.EXPECT().
			GetByEmail(gomock.Any(), input.Email).
			Return(nil, errors.New("connection refused"))

		result, err := s.svc.Register(ctx, input)

		require.Error(t, err)
		assert.Nil(t, result)

		var intErr *apperror.AppError
		require.True(t, errors.As(err, &intErr))
		assert.Equal(t, apperror.CodeInternal, intErr.Code())
	})

	t.Run("error - password hashing fails", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		s.expectTxSuccess()

		s.userRepo.EXPECT().
			GetByEmail(gomock.Any(), input.Email).
			Return(nil, domain.NotFoundError)

		s.passHasher.EXPECT().
			Hash(input.Password).
			Return("", errors.New("bcrypt blew up"))

		result, err := s.svc.Register(ctx, input)

		require.Error(t, err)
		assert.Nil(t, result)

		var intErr *apperror.AppError
		require.True(t, errors.As(err, &intErr))
		assert.Equal(t, apperror.CodeInternal, intErr.Code())
	})

	t.Run("error - userRepo.Create fails", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		s.expectTxSuccess()

		s.userRepo.EXPECT().
			GetByEmail(gomock.Any(), input.Email).
			Return(nil, domain.NotFoundError)

		s.passHasher.EXPECT().
			Hash(input.Password).
			Return(hashedPassword, nil)

		s.userRepo.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			Return(errors.New("database write error"))

		result, err := s.svc.Register(ctx, input)

		require.Error(t, err)
		assert.Nil(t, result)

		var intErr *apperror.AppError
		require.True(t, errors.As(err, &intErr))
		assert.Equal(t, apperror.CodeInternal, intErr.Code())
	})

	t.Run("error - outboxRepo.Create fails", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		s.expectTxSuccess()

		s.userRepo.EXPECT().
			GetByEmail(gomock.Any(), input.Email).
			Return(nil, domain.NotFoundError)

		s.passHasher.EXPECT().
			Hash(input.Password).
			Return(hashedPassword, nil)

		s.userRepo.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			Return(nil)

		s.outboxRepo.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			Return(errors.New("outbox write error"))

		result, err := s.svc.Register(ctx, input)

		require.Error(t, err)
		assert.Nil(t, result)

		var intErr *apperror.AppError
		require.True(t, errors.As(err, &intErr))
		assert.Equal(t, apperror.CodeInternal, intErr.Code())
	})

	t.Run("error - transaction fails to begin/commit", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		txErr := errors.New("failed to commit transaction")
		s.expectTxFailure(txErr)

		result, err := s.svc.Register(ctx, input)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, txErr)
	})

	t.Run("error - race condition duplicate on Create", func(t *testing.T) {
		t.Parallel()

		s := newTestSuite(t)
		ctx := context.Background()

		s.expectTxSuccess()

		s.userRepo.EXPECT().
			GetByEmail(gomock.Any(), input.Email).
			Return(nil, domain.NotFoundError)

		s.passHasher.EXPECT().
			Hash(input.Password).
			Return(hashedPassword, nil)

		s.userRepo.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			Return(domain.AlreadyExistsError)

		result, err := s.svc.Register(ctx, input)

		require.Error(t, err)
		assert.Nil(t, result)

		var bizErr *apperror.AppError
		require.True(t, errors.As(err, &bizErr))
		assert.Equal(t, apperror.CodeAlreadyExists, bizErr.Code())
	})
}

func derefOr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
