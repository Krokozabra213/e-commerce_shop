package providers

import (
	"context"
	"fmt"
	"sync"

	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
)

type Provider interface {
	Supports(name domain.OAuthProvider) bool
	GetAuthURL(state string) string
	ExchangeCode(ctx context.Context, code string) (*domain.OAuthUserInfo, error)
	Name() domain.OAuthProvider
}

type ProviderFactory struct {
	mu        sync.RWMutex
	providers []Provider
}

func (f *ProviderFactory) Register(provider Provider) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.providers = append(f.providers, provider)
}

func (f *ProviderFactory) resolve(name domain.OAuthProvider) (Provider, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	for _, p := range f.providers {
		if p.Supports(name) {
			return p, nil
		}
	}

	return nil, fmt.Errorf("%w: %q", domain.ErrProviderNotFound, name)
}

func (f *ProviderFactory) GetAuthURL(providerName domain.OAuthProvider, state string) (string, error) {
	p, err := f.resolve(providerName)
	if err != nil {
		return "", err
	}

	return p.GetAuthURL(state), nil
}

func (f *ProviderFactory) ExchangeCode(ctx context.Context, providerName domain.OAuthProvider, code string) (*domain.OAuthUserInfo, error) {
	p, err := f.resolve(providerName)
	if err != nil {
		return nil, err
	}

	return p.ExchangeCode(ctx, code)
}

func (f *ProviderFactory) AvailableProviders() []domain.OAuthProvider {
	f.mu.RLock()
	defer f.mu.RUnlock()

	names := make([]domain.OAuthProvider, 0, len(f.providers))
	for _, p := range f.providers {
		names = append(names, p.Name())
	}
	return names
}
