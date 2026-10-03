package auth

import (
	"context"
	"errors"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	httpclient "github.com/Krokozabra213/e-commerce_shop/infra/clients/http"
	"github.com/Krokozabra213/e-commerce_shop/infra/httpx"
	"github.com/Krokozabra213/e-commerce_shop/services/api-gateway/internal/middleware"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
)

const refreshTokenTTL = 720 * time.Hour

type AuthHTTPClient interface {
	Register(ctx context.Context, email, password string) (*httpclient.RegisterResponse, error)
	Login(ctx context.Context, email, password string) (*httpclient.LoginResponse, error)
	Refresh(ctx context.Context, refreshToken string) (*httpclient.RefreshResponse, error)
	Logout(ctx context.Context, refreshToken string) error
	VerifyEmail(ctx context.Context, token string) (*httpclient.VerifyEmailResponse, error)
	SendVerificationEmail(ctx context.Context, userID string) error
	GetPublicKey(ctx context.Context) (*httpclient.GetPublicKeyResponse, error)
	OAuthLogin(ctx context.Context, provider string) (*httpclient.OAuthLoginResponse, error)
	OAuthCallback(ctx context.Context, provider, code, state string) (*httpclient.OAuthCallbackResponse, error)
}

type Handler struct {
	client AuthHTTPClient
}

func NewHandler(client AuthHTTPClient) *Handler {
	return &Handler{client: client}
}

func (h *Handler) Register(c fiber.Ctx) error {
	req := new(registerRequest)

	if err := c.Bind().Body(req); err != nil {
		var valErrs validator.ValidationErrors
		if errors.As(err, &valErrs) {
			return apperror.NewBusiness(apperror.CodeValidation, "Проверьте корректность данных")
		}
		return apperror.NewBusiness(apperror.CodeBadRequest, "Неверный формат запроса")
	}

	resp, err := h.client.Register(c.Context(), req.Email, req.Password)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(resp)
}

func (h *Handler) Login(c fiber.Ctx) error {
	req := new(loginRequest)

	if err := c.Bind().Body(req); err != nil {
		var valErrs validator.ValidationErrors
		if errors.As(err, &valErrs) {
			return apperror.NewBusiness(apperror.CodeValidation, "Проверьте корректность данных")
		}
		return apperror.NewBusiness(apperror.CodeBadRequest, "Неверный формат запроса")
	}

	resp, err := h.client.Login(c.Context(), req.Email, req.Password)
	if err != nil {
		return err
	}

	setRefreshTokenCookie(c, resp.RefreshToken, refreshTokenTTL)

	return c.Status(fiber.StatusOK).JSON(loginResponse{
		AccessToken: resp.AccessToken,
		UserID:      resp.UserID,
	})
}

func (h *Handler) Refresh(c fiber.Ctx) error {
	refreshToken := c.Cookies(httpx.CookieRefreshToken)
	if refreshToken == "" {
		return apperror.NewBusiness(apperror.CodeBadRequest, "missing refresh token cookie")
	}

	resp, err := h.client.Refresh(c.Context(), refreshToken)
	if err != nil {
		return err
	}

	setRefreshTokenCookie(c, resp.RefreshToken, refreshTokenTTL)

	return c.Status(fiber.StatusOK).JSON(tokenResponse{
		AccessToken: resp.AccessToken,
	})
}

func (h *Handler) Logout(c fiber.Ctx) error {
	refreshToken := c.Cookies(httpx.CookieRefreshToken)
	if refreshToken == "" {
		return apperror.NewBusiness(apperror.CodeBadRequest, "missing refresh token cookie")
	}

	if err := h.client.Logout(c.Context(), refreshToken); err != nil {
		return err
	}

	clearRefreshTokenCookie(c)

	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) VerifyEmail(c fiber.Ctx) error {
	token := c.Query("token")
	if token == "" {
		return apperror.NewBusiness(apperror.CodeBadRequest, "token query param is required")
	}

	resp, err := h.client.VerifyEmail(c.Context(), token)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (h *Handler) SendVerificationEmail(c fiber.Ctx) error {

	id, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "unauthorized")
	}
	userID := id.String()

	if err := h.client.SendVerificationEmail(c.Context(), userID); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) OAuthLogin(c fiber.Ctx) error {
	provider := c.Params("provider")
	if provider == "" {
		return apperror.NewBusiness(apperror.CodeBadRequest, "provider param is required")
	}

	resp, err := h.client.OAuthLogin(c.Context(), provider)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (h *Handler) OAuthCallback(c fiber.Ctx) error {
	provider := c.Params("provider")
	if provider == "" {
		return apperror.NewBusiness(apperror.CodeBadRequest, "provider param is required")
	}

	code := c.Query("code")
	state := c.Query("state")
	if code == "" || state == "" {
		return apperror.NewBusiness(apperror.CodeBadRequest, "code and state query params are required")
	}

	resp, err := h.client.OAuthCallback(c.Context(), provider, code, state)
	if err != nil {
		return err
	}

	setRefreshTokenCookie(c, resp.RefreshToken, refreshTokenTTL)

	return c.Status(fiber.StatusOK).JSON(tokenResponse{
		AccessToken: resp.AccessToken,
	})
}
