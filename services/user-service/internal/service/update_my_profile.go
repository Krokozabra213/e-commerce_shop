package service

import (
	"context"
	"errors"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/domain"
	"github.com/google/uuid"
)

func (s *Service) UpdateMyProfile(
	ctx context.Context,
	userID uuid.UUID,
	input domain.UpdateProfileInput,
) (*domain.User, error) {
	user, err := s.repo.UpdateProfile(ctx, userID, input)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, apperror.NewBusiness(apperror.CodeUnauthorized, "Пользователя с таким id не существует")
		}
	}

	return user, nil
}
