package authservice

import (
	"context"
	"time"

	jwtmanager "github.com/Krokozabra213/e-commerce_shop/infra/jwt/manager"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/config"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
	"github.com/google/uuid"
)

//go:generate mockgen -source=service.go -destination=mocks/mock_service.go -package=mocks -typed

type UserRepository interface {
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	Create(ctx context.Context, user *domain.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	ConfirmEmail(ctx context.Context, userID uuid.UUID) error
}

type RefreshTokenRepository interface {
	Create(ctx context.Context, token *domain.RefreshToken) error
	GetByTokenHashForUpdate(ctx context.Context, tokenHash string) (*domain.RefreshToken, error)
	Revoke(ctx context.Context, tokenHash string) error
}

type EmailVerificationRepository interface {
	Create(ctx context.Context, token *domain.EmailVerificationToken) error
	GetByTokenHashForUpdate(ctx context.Context, tokenHash string) (*domain.EmailVerificationToken, error)
	MarkAsUsed(ctx context.Context, tokenHash string) error
}

type OutboxRepository interface {
	Create(ctx context.Context, event *domain.OutboxEvent) error
}

type JWTManager interface {
	GenerateTokens(userID uuid.UUID) (access, refresh string, refreshExp time.Time, err error)
}

type JWTValidator interface {
	ValidateAccess(tokenString string) (*jwtmanager.AccessClaims, error)
	ValidateRefresh(tokenString string) (*jwtmanager.RefreshClaims, error)
}

type JWTKeyStore interface {
	GetPublicKeyPEM() (string, error)
}

type PassHasher interface {
	Hash(password string) (string, error)
	Compare(password, hash string) (bool, error)
}

type TokenHasher interface {
	Hash(value string) string
}

type TxManager interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

type Service struct {
	userRepo              UserRepository
	refreshTokenRepo      RefreshTokenRepository
	emailVerificationRepo EmailVerificationRepository
	outboxRepo            OutboxRepository
	jwtManager            JWTManager
	jwtValidator          JWTValidator
	jwtKeyStore           JWTKeyStore
	txManager             TxManager
	passHasher            PassHasher
	tokenHasher           TokenHasher
	jwtConfig             config.AuthJWTConfig
	emailVerifConfig      config.EmailVerificationConfig
}

func New(
	userRepo UserRepository,
	refreshTokenRepo RefreshTokenRepository,
	emailVerificationRepo EmailVerificationRepository,
	outboxRepo OutboxRepository,
	jwtManager JWTManager,
	jwtValidator JWTValidator,
	jwtKeyStore JWTKeyStore,
	txManager TxManager,
	passHasher PassHasher,
	tokenHasher TokenHasher,
	jwtConfig config.AuthJWTConfig,
	emailVerifConfig config.EmailVerificationConfig,
) *Service {
	return &Service{
		userRepo:              userRepo,
		refreshTokenRepo:      refreshTokenRepo,
		emailVerificationRepo: emailVerificationRepo,
		outboxRepo:            outboxRepo,
		jwtManager:            jwtManager,
		jwtValidator:          jwtValidator,
		jwtKeyStore:           jwtKeyStore,
		txManager:             txManager,
		passHasher:            passHasher,
		tokenHasher:           tokenHasher,
		jwtConfig:             jwtConfig,
		emailVerifConfig:      emailVerifConfig,
	}
}

func stringPtr(s string) *string {
	return &s
}
