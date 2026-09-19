package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/domain"
	"github.com/google/uuid"
)

func (s *Service) CreateFromEvent(
	ctx context.Context,
	userID uuid.UUID,
	email string,
	roles []string,
	createdAt time.Time,
) error {

	_, err := s.repo.GetByID(ctx, userID)
	if err == nil {
		s.logger.Info("user already exists, skipping creation (idempotent)",
			"user_id", userID,
			"email", email,
		)
		return nil
	}
	if !errors.Is(err, domain.NotFoundError) {
		return fmt.Errorf("check user existence: %w", err)
	}

	user := &domain.User{
		ID:        userID,
		Email:     email,
		CreatedAt: createdAt,
	}

	err = s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := s.repo.Create(ctx, user); err != nil {
			if errors.Is(err, domain.AlreadyExistsError) {
				s.logger.Info("user created by another instance, skipping",
					"user_id", userID,
				)
				return nil
			}
			return fmt.Errorf("create user: %w", err)
		}

		for _, r := range roles {
			role := domain.Role(r)
			if !role.IsValid() {
				s.logger.Warn("invalid role in event, skipping",
					"user_id", userID,
					"role", r,
				)
				continue
			}

			if err := s.repo.AddRole(ctx, userID, role); err != nil {
				if errors.Is(err, domain.AlreadyExistsError) {
					continue
				}
				return fmt.Errorf("add role %s: %w", r, err)
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	s.logger.Info("user profile created from event",
		"user_id", userID,
		"email", email,
		"roles", roles,
	)

	return nil
}
