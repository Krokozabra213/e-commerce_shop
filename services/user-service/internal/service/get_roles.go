package service

import (
	"context"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/domain"
	"github.com/google/uuid"
)

func (s *Service) GetRoles(ctx context.Context, userID uuid.UUID) ([]domain.Role, error) {
	roles, err := s.repo.GetRoles(ctx, userID)
	if err != nil {
		return nil, apperror.NewInternal("repo.GetRoles", err, "Что-то пошло не так", nil)
	}

	return roles, nil
}
