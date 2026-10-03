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
				return nil
			}
			return fmt.Errorf("create user: %w", err)
		}

		for _, r := range roles {
			role := domain.Role(r)
			if !role.IsValid() {
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

	return nil
}
