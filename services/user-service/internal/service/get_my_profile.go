package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/domain"
	"github.com/google/uuid"
)

func (s *Service) GetMyProfile(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, apperror.NewBusiness(apperror.CodeUnauthorized, "Пользователя с таким id не существует")
		}
		return nil, fmt.Errorf("get my profile: %w", err)
	}

	return user, nil
}
