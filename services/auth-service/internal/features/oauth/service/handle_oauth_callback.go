package oauthservice

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
	"github.com/google/uuid"
)

func (s *OAuthService) HandleOAuthCallback(ctx context.Context, input OAuthCallbackInput) (*OAuthCallbackOutput, error) {
	valid, err := s.stateStore.Validate(ctx, input.State)
	if err != nil {
		return nil, apperror.NewInternal("stateStore.Validate", err, "Не удалось валидировать state токен", nil)
	}
	if !valid {
		return nil, apperror.NewBusiness(apperror.CodeUnauthorized, "Невалидный state токен")
	}

	_ = s.stateStore.Delete(ctx, input.State)

	oauthUserInfo, err := s.providerFactory.ExchangeCode(ctx, input.Provider, input.Code)
	if err != nil {
		if errors.Is(err, domain.ErrProviderNotFound) {
			return nil, apperror.NewBusiness(apperror.CodeBadRequest, fmt.Sprintf("Провайдер %q не поддерживается", input.Provider))
		}
		return nil, apperror.NewInternal("providerFactory.ExchangeCode", err, "Не удалось обменять код на токен", nil)
	}

	var user *domain.User
	var isNewUser bool

	err = s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		oauthAccount, err := s.oAuthRepository.GetByProviderAndProviderUserID(
			ctx,
			input.Provider,
			oauthUserInfo.ProviderUserID,
		)

		if err != nil && !errors.Is(err, domain.NotFoundError) {
			return apperror.NewInternal("oauthAccountRepo.GetByProviderAndProviderUserID", err, "Ошибка поиска OAuth аккаунта", nil)
		}

		if oauthAccount != nil {
			user, err = s.userRepository.GetByID(ctx, oauthAccount.UserID)
			if err != nil {
				if errors.Is(err, domain.NotFoundError) {
					return apperror.NewBusiness(apperror.CodeNotFound, "Пользователь не найден")
				}
				return apperror.NewInternal("userRepo.GetByID", err, "Ошибка получения пользователя", nil)
			}
			isNewUser = false
			return nil

		} else {

			existingUser, err := s.userRepository.GetByEmail(ctx, oauthUserInfo.Email)
			if err != nil && !errors.Is(err, domain.NotFoundError) {
				return apperror.NewInternal("userRepo.GetByEmail", err, "Ошибка поиска пользователя", nil)
			}

			if existingUser != nil {
				user = existingUser
				isNewUser = false
			} else {
				now := time.Now()
				user = &domain.User{
					ID:               uuid.New(),
					Email:            oauthUserInfo.Email,
					EmailConfirmedAt: &now,
					CreatedAt:        now,
					UpdatedAt:        now,
				}

				if err := s.userRepository.Create(ctx, user); err != nil {
					return apperror.NewInternal("userRepo.Create", err, "Ошибка создания пользователя", nil)
				}
				isNewUser = true

				event := &domain.OutboxEvent{
					ID:            uuid.New(),
					AggregateType: "user",
					AggregateID:   user.ID,
					EventType:     "user.created",
					Payload: map[string]interface{}{
						"id":         user.ID.String(),
						"email":      user.Email,
						"roles":      []string{"ROLE_USER"},
						"source":     "oauth",
						"provider":   input.Provider.String(),
						"created_at": now.Format(time.RFC3339),
					},
					CreatedAt: now,
				}

				if err := s.outboxRepository.Create(ctx, event); err != nil {
					return apperror.NewInternal("outboxRepo.Create", err, "Что-то пошло не так", nil)
				}
			}

			newOAuthAccount := &domain.OauthAccount{
				ID:             uuid.New(),
				UserID:         user.ID,
				Provider:       input.Provider.String(),
				ProviderUserID: oauthUserInfo.ProviderUserID,
				CreatedAt:      time.Now(),
			}

			if err := s.oAuthRepository.Create(ctx, newOAuthAccount); err != nil {
				return apperror.NewInternal("oauthAccountRepo.Create", err, "Ошибка создания OAuth аккаунта", nil)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	accessToken, refreshTokenStr, tokenExpiresAt, err := s.jwtManager.GenerateTokens(user.ID)
	if err != nil {
		return nil, apperror.NewInternal("jwtManager.GenerateTokens", err, "Ошибка генерации токенов", nil)
	}

	refreshToken := &domain.RefreshToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		TokenHash: s.tokenHasher.Hash(refreshTokenStr),
		ExpiresAt: tokenExpiresAt,
		Revoked:   false,
		CreatedAt: time.Now(),
	}

	if err := s.refreshTokenRepository.Create(ctx, refreshToken); err != nil {
		return nil, apperror.NewInternal("refreshTokenRepo.Create", err, "Ошибка сохранения refresh токена", nil)
	}

	return &OAuthCallbackOutput{
		AccessToken:  accessToken,
		RefreshToken: refreshTokenStr,
		IsNewUser:    isNewUser,
		User:         user,
	}, nil
}
