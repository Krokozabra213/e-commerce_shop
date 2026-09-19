package service

import (
	"context"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/domain"
)

func (s *Service) ListUsers(ctx context.Context, filter domain.ListUsersFilter) ([]*domain.User, int, error) {
	users, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, 0, apperror.NewInternal("repo.List", err, "Не удалось получить пользователей", nil)
	}

	return users, total, nil
}
