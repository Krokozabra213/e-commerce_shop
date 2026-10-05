package authservice

import (
	"context"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
)

func (s *Service) GetPublicKey(_ context.Context) (*GetPublicKeyOutput, error) {
	pemKey, err := s.jwtKeyStore.GetPublicKeyPEM()
	if err != nil {
		return nil, apperror.NewInternal("jwtKeyStore.GetPublicKeyPEM", err, "Не удалось получить публичный ключ", nil)
	}

	return &GetPublicKeyOutput{
		PublicKey: pemKey,
	}, nil
}
