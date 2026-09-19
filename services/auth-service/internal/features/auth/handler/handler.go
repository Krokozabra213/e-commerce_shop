package authhandler

import (
	"context"
	"log/slog"

	authservice "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/auth/service"
)

type AuthService interface {
	Register(ctx context.Context, input authservice.RegisterInput) (*authservice.RegisterOutput, error)
	VerifyEmail(ctx context.Context, input authservice.VerifyEmailInput) error
	Login(ctx context.Context, input authservice.LoginInput) (*authservice.LoginOutput, error)
	Refresh(ctx context.Context, input authservice.RefreshInput) (*authservice.RefreshOutput, error)
	Logout(ctx context.Context, input authservice.LogoutInput) error
	SendVerificationEmail(ctx context.Context, input authservice.SendVerificationEmailInput) error
	GetPublicKey(ctx context.Context) (*authservice.GetPublicKeyOutput, error)
}

type Handler struct {
	authService AuthService
	log         *slog.Logger
}

func New(authService AuthService, log *slog.Logger) *Handler {
	return &Handler{
		authService: authService,
		log:         log,
	}
}
