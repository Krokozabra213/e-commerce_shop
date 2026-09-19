package inframiddleware

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/gofiber/fiber/v3"
)

type ErrorHandlerMiddleware struct {
	logger *slog.Logger
}

func NewErrorHandler(logger *slog.Logger) *ErrorHandlerMiddleware {
	return &ErrorHandlerMiddleware{
		logger: logger,
	}
}

func (h *ErrorHandlerMiddleware) Handle(ctx fiber.Ctx, err error) error {
	code := http.StatusInternalServerError
	message := "Internal server error"

	var e *fiber.Error
	if errors.As(err, &e) {
		code = e.Code
		message = e.Message
	}

	var apiErr *apperror.AppError
	if errors.As(err, &apiErr) {
		code = mapKindToStatus(apiErr.Code())
		message = apiErr.Message()
	}

	if code >= http.StatusInternalServerError {
		h.logger.Error("internal server error",
			slog.Any("error", err),
			slog.String("path", ctx.Path()),
			slog.String("method", ctx.Method()),
		)

		ctx.Status(code)
		return ctx.JSON(fiber.Map{
			"error": "Internal server error",
		})
	}

	ctx.Status(code)
	return ctx.JSON(fiber.Map{
		"error": message,
	})
}

func mapKindToStatus(code apperror.Code) int {
	switch code {
	case apperror.CodeBadRequest, apperror.CodeValidation:
		return http.StatusBadRequest
	case apperror.CodeAlreadyExists:
		return http.StatusConflict
	case apperror.CodeUnauthorized:
		return http.StatusUnauthorized
	case apperror.CodeNotFound:
		return http.StatusNotFound
	case apperror.CodeForbidden:
		return http.StatusForbidden

	default:
		return http.StatusInternalServerError
	}
}
