package authhandler

import (
	"errors"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	authservice "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/auth/service"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
)

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type RefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) RefreshToken(c fiber.Ctx) error {
	req := new(RefreshRequest)

	if err := c.Bind().Body(req); err != nil {
		var valErrs validator.ValidationErrors
		if errors.As(err, &valErrs) {
			return apperror.NewBusiness(apperror.CodeValidation, "Проверьте правильность заполнения полей")
		}
		return apperror.NewBusiness(apperror.CodeBadRequest, "Неверный формат запроса")
	}

	result, err := h.authService.Refresh(c.Context(), authservice.RefreshInput{
		RefreshToken: req.RefreshToken,
	})
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(RefreshResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
	})
}
