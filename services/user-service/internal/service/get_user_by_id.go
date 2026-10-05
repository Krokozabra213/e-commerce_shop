package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/domain"
	"github.com/google/uuid"
)

func (s *Service) GetUserByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, apperror.NewBusiness(apperror.CodeUnauthorized, "Пользователя с таким id не существует")
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}

	return user, nil
}
