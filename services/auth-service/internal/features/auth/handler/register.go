package authhandler

import (
	"errors"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	authservice "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/auth/service"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
)

type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

type RegisterResponse struct {
	UserID string `json:"user_id"`
}

func (h *Handler) Register(c fiber.Ctx) error {
	req := new(RegisterRequest)

	if err := c.Bind().Body(req); err != nil {
		var valErrs validator.ValidationErrors
		if errors.As(err, &valErrs) {
			return apperror.NewBusiness(apperror.CodeValidation, "Проверьте правильность заполнения полей")
		}
		return apperror.NewBusiness(apperror.CodeBadRequest, "Неверный формат запроса")
	}

	result, err := h.authService.Register(c.Context(), authservice.RegisterInput{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(RegisterResponse{
		UserID: result.UserID.String(),
	})
}
