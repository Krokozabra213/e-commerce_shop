package authhandler

import (
	"errors"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	authservice "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/auth/service"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type SendVerificationEmailRequest struct {
	UserID string `json:"user_id" validate:"required,uuid4"`
}

func (h *Handler) SendVerificationEmail(c fiber.Ctx) error {
	req := new(SendVerificationEmailRequest)

	if err := c.Bind().Body(req); err != nil {
		var valErrs validator.ValidationErrors
		if errors.As(err, &valErrs) {
			return apperror.NewBusiness(apperror.CodeValidation, "Проверьте правильность заполнения полей")
		}
		return apperror.NewBusiness(apperror.CodeBadRequest, "Неверный формат запроса")
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		return apperror.NewBusiness(apperror.CodeBadRequest, "Неверный формат user_id")
	}

	if err := h.authService.SendVerificationEmail(c.Context(), authservice.SendVerificationEmailInput{
		UserID: userID,
	}); err != nil {
		return err
	}

	return c.Status(fiber.StatusNoContent).Send(nil)
}
