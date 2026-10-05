package authfeature

import (
	"crypto/rsa"
	"time"

	jwtkeystore "github.com/Krokozabra213/e-commerce_shop/infra/jwt/keystore"
	jwtmanager "github.com/Krokozabra213/e-commerce_shop/infra/jwt/manager"
	jwtvalidator "github.com/Krokozabra213/e-commerce_shop/infra/jwt/validator"
	txmanager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/config"
	authhandler "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/auth/handler"
	emailverificationRepo "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/auth/repository/postgres/email-verification"
	refreshtokenRepo "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/auth/repository/postgres/refresh-token"
	userRepo "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/auth/repository/postgres/user"
	authservice "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/auth/service"
	outboxRepo "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/outbox/repository/postgres"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/security"
	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type Module struct {
	Service *authservice.Service
	handler *authhandler.Handler
}

type Dependencies struct {
	PGXPool       *pgxpool.Pool
	Config        *config.Config
	RSAPrivateKey *rsa.PrivateKey
}

func New(deps *Dependencies) *Module {
	userRepository := userRepo.NewPostgresUserRepository(deps.PGXPool)
	refreshTokenRepository := refreshtokenRepo.NewPostgresRefreshTokenRepository(deps.PGXPool)
	emailVerificationRepository := emailverificationRepo.NewPostgresEmailVerificationRepository(deps.PGXPool)
	outboxRepository := outboxRepo.NewPostgresOutboxRepository(deps.PGXPool)

	txManager := txmanager.NewPgTxManager(deps.PGXPool)
	passHasher := security.NewPasswordHasher(bcrypt.DefaultCost)
	tokenHasher := security.NewHashingService(deps.Config.App.Secret)
	jwtManager, err := jwtmanager.NewRS256Manager(deps.RSAPrivateKey, deps.Config.AuthJWT.AccessTTL, deps.Config.AuthJWT.RefreshTTL, deps.Config.AuthJWT.Issuer)
	if err != nil {
		panic(err)
	}
	publicKey := deps.RSAPrivateKey.PublicKey
	jwtValidator, err := jwtvalidator.NewRS256Validator(&publicKey, time.Second*30, deps.Config.AuthJWT.Issuer)
	if err != nil {
		panic(err)
	}
	jwtKeyStore, err := jwtkeystore.NewRS256KeyStore(deps.RSAPrivateKey, deps.Config.AuthJWT.KeyID)
	if err != nil {
		panic(err)
	}

	svc := authservice.New(
		userRepository,
		refreshTokenRepository,
		emailVerificationRepository,
		outboxRepository,
		jwtManager,
		jwtValidator,
		jwtKeyStore,
		txManager,
		passHasher,
		tokenHasher,
		deps.Config.AuthJWT,
		deps.Config.EmailVerification,
	)

	h := authhandler.New(svc)

	return &Module{
		Service: svc,
		handler: h,
	}
}

func (m *Module) HTTPv1() func(fiber.Router) {
	return func(router fiber.Router) {
		auth := router.Group("/api/v1/auth")

		auth.Post("/register", m.handler.Register)
		auth.Post("/login", m.handler.Login)
		auth.Post("/refresh", m.handler.RefreshToken)
		auth.Post("/logout", m.handler.Logout)
		auth.Get("/verify-email", m.handler.VerifyEmail)
		auth.Post("/resend-verification", m.handler.SendVerificationEmail)
		auth.Get("/public-key", m.handler.GetPublicKey)
	}
}
