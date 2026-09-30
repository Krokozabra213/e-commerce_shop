package authservice

import (
	"context"
	"errors"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	infrakafka "github.com/Krokozabra213/e-commerce_shop/infra/kafka"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/dto"
	"github.com/google/uuid"
)

func (s *Service) Register(ctx context.Context, input RegisterInput) (*RegisterOutput, error) {
	var userID uuid.UUID

	err := s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		existingUser, err := s.userRepo.GetByEmail(ctx, input.Email)
		if err != nil && !errors.Is(err, domain.NotFoundError) {
			return apperror.NewInternal("userRepo.GetByEmail", err, "Что-то пошло не так", nil)
		}
		if existingUser != nil {
			return apperror.NewBusiness(apperror.CodeAlreadyExists, "Пользователь уже существует")
		}

		passwordHash, err := s.passHasher.Hash(input.Password)
		if err != nil {
			return apperror.NewInternal("bcrypt.GenerateFromPassword", err, "Что-то пошло не так", nil)
		}

		now := time.Now()
		user := &domain.User{
			ID:           uuid.New(),
			Email:        input.Email,
			PasswordHash: stringPtr(passwordHash),
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		if err := s.userRepo.Create(ctx, user); err != nil {
			if errors.Is(err, domain.AlreadyExistsError) {
				return apperror.NewBusiness(apperror.CodeAlreadyExists, "Пользователь уже существует")
			}
			return apperror.NewInternal("userRepo.Create", err, "Что-то пошло не так", nil)
		}

		userID = user.ID

		payload := dto.UserCreatedPayload{
			ID:        user.ID.String(),
			Email:     user.Email,
			Roles:     []string{"ROLE_USER"},
			CreatedAt: user.CreatedAt,
		}

		payloadMap, err := dto.ToMap(payload)
		if err != nil {
			return apperror.NewInternal("marshal user payload", err, "Что-то пошло не так", nil)
		}

		event := &domain.OutboxEvent{
			ID:            uuid.New(),
			AggregateType: "user",
			AggregateID:   user.ID,
			EventType:     infrakafka.TopicUserCreated,
			Payload:       payloadMap,
			CreatedAt:     now,
		}

		if err := s.outboxRepo.Create(ctx, event); err != nil {
			return apperror.NewInternal("outboxRepo.Create", err, "Что-то пошло не так", nil)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &RegisterOutput{UserID: userID}, nil
}
