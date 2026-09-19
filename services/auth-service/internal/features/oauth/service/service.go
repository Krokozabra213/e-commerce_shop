package oauthservice

import (
	"context"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
	"github.com/google/uuid"
)

type ProviderFactory interface {
	GetAuthURL(providerName domain.OAuthProvider, state string) (string, error)
	ExchangeCode(ctx context.Context, providerName domain.OAuthProvider, code string) (*domain.OAuthUserInfo, error)
	AvailableProviders() []domain.OAuthProvider
}

type OAuthStateStore interface {
	Set(ctx context.Context, state string, ttl time.Duration) error
	Validate(ctx context.Context, state string) (bool, error)
	Delete(ctx context.Context, state string) error
}

type TxManager interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

type OAuthRepository interface {
	GetByProviderAndProviderUserID(ctx context.Context, provider domain.OAuthProvider, providerUserID string) (*domain.OauthAccount, error)
	Create(ctx context.Context, account *domain.OauthAccount) error
}

type UserRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	Create(ctx context.Context, user *domain.User) error
}

type JWTManager interface {
	GenerateTokens(userID uuid.UUID) (access, refresh string, refreshExp time.Time, err error)
}

type TokenHasher interface {
	Hash(value string) string
}

type RefreshTokenRepository interface {
	Create(ctx context.Context, token *domain.RefreshToken) error
}

type OutboxRepository interface {
	Create(ctx context.Context, event *domain.OutboxEvent) error
}

type OAuthService struct {
	providerFactory        ProviderFactory
	stateStore             OAuthStateStore
	stateExpiration        time.Duration
	txManager              TxManager
	oAuthRepository        OAuthRepository
	userRepository         UserRepository
	jwtManager             JWTManager
	tokenHasher            TokenHasher
	refreshTokenRepository RefreshTokenRepository
	outboxRepository       OutboxRepository
}

func NewOAuthService(
	providerFactory ProviderFactory,
	stateStore OAuthStateStore,
	stateExpiration time.Duration,
	txManager TxManager,
	oAuthRepo OAuthRepository,
	userRepository UserRepository,
	jwtManager JWTManager,
	tokenHasher TokenHasher,
	refreshTokenRepository RefreshTokenRepository,
	outboxRepository OutboxRepository,
) *OAuthService {
	return &OAuthService{
		providerFactory:        providerFactory,
		stateStore:             stateStore,
		stateExpiration:        stateExpiration,
		txManager:              txManager,
		oAuthRepository:        oAuthRepo,
		userRepository:         userRepository,
		jwtManager:             jwtManager,
		tokenHasher:            tokenHasher,
		refreshTokenRepository: refreshTokenRepository,
		outboxRepository:       outboxRepository,
	}
}

func (s *OAuthService) GetAvailableProviders(ctx context.Context) (*GetAvailableProvidersOutput, error) {
	providers := s.providerFactory.AvailableProviders()
	return &GetAvailableProvidersOutput{
		Providers: providers,
	}, nil
}
