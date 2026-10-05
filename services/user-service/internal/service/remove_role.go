package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/domain"
	"github.com/google/uuid"
)

func (s *Service) RemoveRole(ctx context.Context, userID uuid.UUID, role domain.Role) error {
	err := s.repo.RemoveRole(ctx, userID, role)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return apperror.NewBusiness(apperror.CodeBadRequest, "Пользователя с таким id или ролью не существует")
		}
		return fmt.Errorf("remove role: %w", err)
	}

	return nil
}
