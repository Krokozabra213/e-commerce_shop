package oauthservice

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
)

func (s *OAuthService) GetOAuthLoginURL(ctx context.Context, input GetOAuthLoginURLInput) (*GetOAuthLoginURLOutput, error) {
	state, err := generateSecureState()
	if err != nil {
		return nil, apperror.NewInternal("generateSecureState", err, "Не удалось сгенерировать state токен", nil)
	}

	if err := s.stateStore.Set(ctx, state, s.stateExpiration); err != nil {
		return nil, apperror.NewInternal("stateStore.Set", err, "Не удалось сохранить state токен", nil)
	}

	authURL, err := s.providerFactory.GetAuthURL(input.Provider, state)
	if err != nil {
		if errors.Is(err, domain.ErrProviderNotFound) {
			return nil, apperror.NewBusiness(apperror.CodeBadRequest, fmt.Sprintf("Провайдер %q не поддерживается", input.Provider))
		}
		return nil, apperror.NewInternal("providerFactory.GetAuthURL", err, "Не удалось получить URL авторизации", nil)
	}

	return &GetOAuthLoginURLOutput{
		AuthURL: authURL,
		State:   state,
	}, nil
}

func generateSecureState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
