package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/domain"
	"github.com/google/uuid"
)

func (s *Service) AddRole(ctx context.Context, userID uuid.UUID, role domain.Role) error {
	err := s.repo.AddRole(ctx, userID, role)
	if err != nil {
		if errors.Is(err, domain.NotFoundError) {
			return apperror.NewBusiness(apperror.CodeBadRequest, "Пользователя с таким id не существует")
		}
		if errors.Is(err, domain.AlreadyExistsError) {
			return apperror.NewBusiness(apperror.CodeBadRequest, "У пользователя уже есть эта роль")
		}
		return fmt.Errorf("add role: %w", err)
	}

	return nil
}
