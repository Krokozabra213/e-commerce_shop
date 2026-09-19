package authservice

import (
	"context"
	"errors"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
	"github.com/google/uuid"
)

func (s *Service) Refresh(ctx context.Context, input RefreshInput) (*RefreshOutput, error) {
	claims, err := s.jwtValidator.ValidateRefresh(input.RefreshToken)
	if err != nil {
		return nil, apperror.NewBusiness(apperror.CodeUnauthorized, "Токен пользователя невалиден")
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, apperror.NewBusiness(apperror.CodeUnauthorized, "ID пользователя невалиден")
	}

	newAccessToken, newRefreshTokenStr, newTokenExpiresAt, err := s.jwtManager.GenerateTokens(userID)
	if err != nil {
		return nil, apperror.NewInternal("jwtManager.GenerateTokens", err, "Ошибка генерации токенов", nil)
	}

	oldTokenHash := s.tokenHasher.Hash(input.RefreshToken)
	newTokenHash := s.tokenHasher.Hash(newRefreshTokenStr)

	err = s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		storedToken, err := s.refreshTokenRepo.GetByTokenHashForUpdate(ctx, oldTokenHash)
		if err != nil {
			if errors.Is(err, domain.NotFoundError) {
				return apperror.NewBusiness(apperror.CodeUnauthorized, "Токен пользователя ненайден")
			}
			return apperror.NewInternal("refreshTokenRepo.GetByTokenHashForUpdate", err, "Что-то пошло не так", nil)
		}

		if storedToken.UserID != userID {
			return apperror.NewInternal("refreshTokenRepo.GetByTokenHashForUpdate", nil, "Владелец токена не совпадает", nil)
		}

		if storedToken.Revoked {
			return apperror.NewBusiness(apperror.CodeUnauthorized, "Токен пользователя уже отозван")
		}

		if storedToken.ExpiresAt.Before(time.Now()) {
			return apperror.NewBusiness(apperror.CodeUnauthorized, "Токен пользователя истёк")
		}

		if err := s.refreshTokenRepo.Revoke(ctx, oldTokenHash); err != nil {
			return apperror.NewInternal("refreshTokenRepo.Revoke", err, "Что-то пошло не так", nil)
		}

		newRefreshToken := &domain.RefreshToken{
			ID:        uuid.New(),
			UserID:    userID,
			TokenHash: newTokenHash,
			ExpiresAt: newTokenExpiresAt,
			Revoked:   false,
			CreatedAt: time.Now(),
		}

		if err := s.refreshTokenRepo.Create(ctx, newRefreshToken); err != nil {
			return apperror.NewInternal("refreshTokenRepo.Create", err, "Что-то пошло не так", nil)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &RefreshOutput{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshTokenStr,
	}, nil
}
