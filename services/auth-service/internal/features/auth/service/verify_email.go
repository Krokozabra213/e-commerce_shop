package authservice

import (
	"context"
	"errors"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
)

func (s *Service) VerifyEmail(ctx context.Context, input VerifyEmailInput) error {
	tokenHash := s.tokenHasher.Hash(input.Token)

	return s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		token, err := s.emailVerificationRepo.GetByTokenHashForUpdate(ctx, tokenHash)
		if err != nil {
			if errors.Is(err, domain.NotFoundError) {
				return apperror.NewBusiness(apperror.CodeBadRequest, "Ссылка недействительна")
			}
			return apperror.NewInternal("emailVerificationRepo.GetByTokenHashForUpdate", err, "Токен не найден", nil)
		}

		if token.UsedAt != nil {
			return apperror.NewBusiness(apperror.CodeBadRequest, "Ссылка уже была использована")
		}

		if token.ExpiresAt.Before(time.Now()) {
			return apperror.NewBusiness(apperror.CodeBadRequest, "Срок действия ссылки истёк")
		}

		if err := s.emailVerificationRepo.MarkAsUsed(ctx, tokenHash); err != nil {
			return apperror.NewInternal("emailVerificationRepo.MarkAsUsed", err, "Что-то пошло не так", nil)
		}

		if err := s.userRepo.ConfirmEmail(ctx, token.UserID); err != nil {
			return apperror.NewInternal("userRepo.ConfirmEmail", err, "Что-то пошло не так", nil)
		}

		return nil
	})
}
