package authservice

import (
	"context"
	"errors"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
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
			PasswordHash: stringPtr(string(passwordHash)),
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

		event := &domain.OutboxEvent{
			ID:            uuid.New(),
			AggregateType: "user",
			AggregateID:   user.ID,
			EventType:     "user.created",
			Payload: map[string]interface{}{
				"id":         user.ID.String(),
				"email":      user.Email,
				"roles":      []string{"ROLE_USER"},
				"created_at": user.CreatedAt.Format(time.RFC3339),
			},
			CreatedAt: now,
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
