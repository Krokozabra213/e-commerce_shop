package authservice

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
	"github.com/google/uuid"
)

func (s *Service) Login(ctx context.Context, input LoginInput) (*LoginOutput, error) {
	user, err := s.userRepo.GetByEmail(ctx, input.Email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, apperror.NewBusiness(apperror.CodeUnauthorized, "Пользователя с таким email или паролем не существует")
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	if user.PasswordHash == nil {
		return nil, apperror.NewBusiness(apperror.CodeUnauthorized, "Этот аккаунт привязан к входу через сторонний сервис. Войдите через него или восстановите пароль.")
	}

	passwordHash := *user.PasswordHash
	same, err := s.passHasher.Compare(input.Password, passwordHash)
	if err != nil {
		return nil, apperror.NewInternal("passHasher.Compare", err, "Что-то пошло не так", nil)
	}

	if !same {
		return nil, apperror.NewBusiness(apperror.CodeUnauthorized, "Пользователя с таким email или паролем не существует")
	}

	accessToken, refreshTokenStr, refreshExp, err := s.jwtManager.GenerateTokens(user.ID)
	if err != nil {
		return nil, apperror.NewInternal("jwtManager.GenerateTokens", err, "Что-то пошло не так", nil)
	}

	tokenHash := s.tokenHasher.Hash(refreshTokenStr)

	refreshToken := &domain.RefreshToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: refreshExp,
		Revoked:   false,
		CreatedAt: time.Now(),
	}

	if err := s.refreshTokenRepo.Create(ctx, refreshToken); err != nil {
		return nil, apperror.NewInternal("refreshTokenRepo.Create", err, "не удалось сохранить сессию", nil)
	}

	return &LoginOutput{
		AccessToken:  accessToken,
		RefreshToken: refreshTokenStr,
		UserID:       user.ID,
	}, nil
}
