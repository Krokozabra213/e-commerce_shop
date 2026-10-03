package oauthhandler

import (
	"context"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
	oauthservice "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/oauth/service"
	"github.com/gofiber/fiber/v3"
)

type OauthService interface {
	GetOAuthLoginURL(ctx context.Context, input oauthservice.GetOAuthLoginURLInput) (*oauthservice.GetOAuthLoginURLOutput, error)
	HandleOAuthCallback(ctx context.Context, input oauthservice.OAuthCallbackInput) (*oauthservice.OAuthCallbackOutput, error)
}

type Handler struct {
	oauthService OauthService
}

func New(oauthService OauthService) *Handler {
	return &Handler{
		oauthService: oauthService,
	}
}

type loginResponse struct {
	AuthURL string `json:"auth_url"`
	State   string `json:"state"`
}

func (h *Handler) Login(c fiber.Ctx) error {
	providerStr := c.Params("provider")
	if providerStr == "" {
		return apperror.NewBusiness(apperror.CodeBadRequest, "Провайдер не указан в URL")
	}

	provider := domain.OAuthProvider(providerStr)
	if provider.IsValid() == false {
		return apperror.NewBusiness(apperror.CodeValidation, "Невалидный провайдер")
	}

	out, err := h.oauthService.GetOAuthLoginURL(c.Context(), oauthservice.GetOAuthLoginURLInput{
		Provider: provider,
	})
	if err != nil {
		return err
	}

	return c.JSON(loginResponse{
		AuthURL: out.AuthURL,
		State:   out.State,
	})
}

type callbackRequest struct {
	Code  string `query:"code" validate:"required"`
	State string `query:"state" validate:"required"`
}

func (h *Handler) Callback(c fiber.Ctx) error {
	provider := c.Params("provider")
	if provider == "" {
		return apperror.NewBusiness(apperror.CodeBadRequest, "Провайдер не указан")
	}

	validProvider := domain.OAuthProvider(provider)
	if validProvider.IsValid() == false {
		return apperror.NewBusiness(apperror.CodeValidation, "Невалидный провайдер")
	}

	var req callbackRequest
	if err := c.Bind().Query(&req); err != nil || req.Code == "" || req.State == "" {
		return apperror.NewBusiness(apperror.CodeBadRequest, "Отсутствуют параметры code или state")
	}

	out, err := h.oauthService.HandleOAuthCallback(c.Context(), oauthservice.OAuthCallbackInput{
		Provider: validProvider,
		Code:     req.Code,
		State:    req.State,
	})
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"access_token":  out.AccessToken,
		"refresh_token": out.RefreshToken,
	})
}
