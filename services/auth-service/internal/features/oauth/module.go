package oauthfeature

import (
	"crypto/rsa"
	"time"

	jwtmanager "github.com/Krokozabra213/e-commerce_shop/infra/jwt/manager"
	tx_manager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/config"
	refreshtokenRepo "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/auth/repository/postgres/refresh-token"
	userRepo "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/auth/repository/postgres/user"
	oauthhandler "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/oauth/handler"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/oauth/providers"
	githubProvider "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/oauth/providers/github"
	googleProvider "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/oauth/providers/google"
	oauthrepo "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/oauth/repository/postgres"
	stateStore "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/oauth/repository/redis-state-store"
	oauthservice "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/oauth/service"
	outboxRepo "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/outbox/repository/postgres"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/security"
	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Module struct {
	Service *oauthservice.OAuthService
	handler *oauthhandler.Handler
}

type Dependencies struct {
	PGXPool       *pgxpool.Pool
	RedisClient   redis.UniversalClient
	Config        *config.Config
	RSAPrivateKey *rsa.PrivateKey
}

func New(deps *Dependencies) *Module {

	userRepository := userRepo.NewPostgresUserRepository(deps.PGXPool)
	refreshTokenRepository := refreshtokenRepo.NewPostgresRefreshTokenRepository(deps.PGXPool)
	outboxRepository := outboxRepo.NewPostgresOutboxRepository(deps.PGXPool)
	oauthRepo := oauthrepo.NewPostgresOAuthRepository(deps.PGXPool)
	stateStore := stateStore.NewRedisStateStore(deps.RedisClient)

	txManager := tx_manager.NewPgTxManager(deps.PGXPool)
	jwtManager, err := jwtmanager.NewRS256Manager(deps.RSAPrivateKey, deps.Config.AuthJWT.AccessTTL, deps.Config.AuthJWT.RefreshTTL, deps.Config.AuthJWT.Issuer)
	if err != nil {
		panic(err)
	}
	tokenHasher := security.NewHashingService(deps.Config.App.Secret)

	googleProvider := googleProvider.New(
		deps.Config.Oauth.Google.ClientID,
		deps.Config.Oauth.Google.ClientSecret,
		deps.Config.Oauth.Google.RedirectURL,
	)

	githubProvider := githubProvider.New(
		deps.Config.Oauth.GitHub.ClientID,
		deps.Config.Oauth.GitHub.ClientSecret,
		deps.Config.Oauth.GitHub.RedirectURL,
	)

	providerFactory := new(providers.ProviderFactory)
	providerFactory.Register(googleProvider)
	providerFactory.Register(githubProvider)

	svc := oauthservice.NewOAuthService(
		providerFactory,
		stateStore,
		15*time.Minute,
		txManager,
		oauthRepo,
		userRepository,
		jwtManager,
		tokenHasher,
		refreshTokenRepository,
		outboxRepository,
	)

	h := oauthhandler.New(svc)

	return &Module{
		Service: svc,
		handler: h,
	}
}

func (m *Module) HTTPv1() func(fiber.Router) {
	return func(router fiber.Router) {
		oauth := router.Group("/api/v1/auth/oauth")

		oauth.Get("/:provider/login", m.handler.Login)
		oauth.Get("/:provider/callback", m.handler.Callback)
	}
}
