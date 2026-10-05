package authhandler

import (
	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	authservice "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/auth/service"
	"github.com/gofiber/fiber/v3"
)

type VerifyEmailResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func (h *Handler) VerifyEmail(c fiber.Ctx) error {
	token := c.Query("token")

	if token == "" {
		return apperror.NewBusiness(
			apperror.CodeValidation,
			"Параметр token обязателен",
		)
	}
	if len(token) < 32 {
		return apperror.NewBusiness(
			apperror.CodeValidation,
			"Некорректный формат токена",
		)
	}

	err := h.authService.VerifyEmail(c.Context(), authservice.VerifyEmailInput{
		Token: token,
	})
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(VerifyEmailResponse{
		Success: true,
		Message: "Email успешно подтверждён",
	})
}
