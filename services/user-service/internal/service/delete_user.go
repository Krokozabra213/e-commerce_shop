package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/domain"
	"github.com/google/uuid"
)

func (s *Service) DeleteUser(ctx context.Context, id uuid.UUID) error {
	err := s.repo.SoftDelete(ctx, id)
	if err != nil {
		if errors.Is(err, domain.NotFoundError) {
			return apperror.NewBusiness(apperror.CodeUnauthorized, "Пользователя с таким id не существует")
		}
		return fmt.Errorf("delete user: %w", err)
	}

	return nil
}
