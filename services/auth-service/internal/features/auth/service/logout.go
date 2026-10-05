package authservice

import (
	"context"
	"errors"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
)

func (s *Service) Logout(ctx context.Context, input LogoutInput) error {
	tokenHash := s.tokenHasher.Hash(input.RefreshToken)
	if err := s.refreshTokenRepo.Revoke(ctx, tokenHash); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil
		}
		return apperror.NewInternal("refreshTokenRepo.Revoke", err, "Что-то пошло не так", nil)
	}

	return nil
}
