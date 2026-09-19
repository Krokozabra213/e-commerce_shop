package authhandler

import (
	"errors"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	authservice "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/auth/service"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
)

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

func (h *Handler) Logout(c fiber.Ctx) error {
	req := new(LogoutRequest)

	if err := c.Bind().Body(req); err != nil {
		var valErrs validator.ValidationErrors
		if errors.As(err, &valErrs) {
			return apperror.NewBusiness(apperror.CodeValidation, "Проверьте правильность заполнения полей")
		}
		return apperror.NewBusiness(apperror.CodeBadRequest, "Неверный формат запроса")
	}

	if err := h.authService.Logout(c.Context(), authservice.LogoutInput{
		RefreshToken: req.RefreshToken,
	}); err != nil {
		return err
	}

	return c.Status(fiber.StatusNoContent).Send(nil)
}
