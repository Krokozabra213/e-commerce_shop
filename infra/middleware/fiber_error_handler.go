package inframiddleware

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	log "github.com/Krokozabra213/e-commerce_shop/infra/logger"
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

func (h *ErrorHandlerMiddleware) loggerFor(ctx fiber.Ctx) *slog.Logger {
	if l, ok := log.FromContextOK(ctx.Context()); ok {
		return l
	}
	return h.logger
}

func (h *ErrorHandlerMiddleware) Handle(ctx fiber.Ctx, err error) error {
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		return h.handleFiberError(ctx, fiberErr)
	}

	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return h.handleAppError(ctx, appErr)
	}

	return h.handleUnknownError(ctx, err)
}

func (h *ErrorHandlerMiddleware) handleFiberError(ctx fiber.Ctx, err *fiber.Error) error {
	logger := h.loggerFor(ctx)

	if err.Code >= http.StatusInternalServerError {
		logger.ErrorContext(ctx.Context(), "fiber error",
			slog.Int("status", err.Code),
			slog.String("message", err.Message),
			slog.String("path", ctx.Path()),
			slog.String("method", ctx.Method()),
		)
	} else {
		logger.InfoContext(ctx.Context(), "fiber error",
			slog.Int("status", err.Code),
			slog.String("message", err.Message),
			slog.String("path", ctx.Path()),
			slog.String("method", ctx.Method()),
		)
	}

	return ctx.Status(err.Code).JSON(fiber.Map{
		"error": err.Message,
	})
}

func (h *ErrorHandlerMiddleware) handleAppError(ctx fiber.Ctx, appErr *apperror.AppError) error {
	logger := h.loggerFor(ctx)

	statusCode := mapAppCodeToStatus(appErr.Code())
	slogLevel := mapAppLevelToSlog(appErr.LogLevel())

	attrs := []slog.Attr{
		slog.String("path", ctx.Path()),
		slog.String("method", ctx.Method()),
		slog.String("code", appErr.Code().String()),
		slog.Int("status", statusCode),
	}

	if appErr.Op() != "" {
		attrs = append(attrs, slog.String("op", appErr.Op()))
	}

	if appErr.Err() != nil {
		attrs = append(attrs, slog.String("error", appErr.Err().Error()))
	}

	if appErr.Attrs() != nil {
		attrs = append(attrs, appErr.Attrs().ToAttrs()...)
	}

	logger.LogAttrs(ctx.Context(), slogLevel, appErr.Message(), attrs...)

	if statusCode >= http.StatusInternalServerError {
		return ctx.Status(statusCode).JSON(fiber.Map{
			"error": "Internal server error",
		})
	}

	return ctx.Status(statusCode).JSON(fiber.Map{
		"error": appErr.Message(),
	})
}

func (h *ErrorHandlerMiddleware) handleUnknownError(ctx fiber.Ctx, err error) error {
	logger := h.loggerFor(ctx)

	logger.ErrorContext(ctx.Context(), "unhandled error",
		slog.String("path", ctx.Path()),
		slog.String("method", ctx.Method()),
		slog.Any("error", err),
	)

	return ctx.Status(http.StatusInternalServerError).JSON(fiber.Map{
		"error": "Internal server error",
	})
}

func mapAppCodeToStatus(code apperror.Code) int {
	if status, ok := statusMapping[code]; ok {
		return status
	}
	return http.StatusInternalServerError
}

var statusMapping = map[apperror.Code]int{
	apperror.CodeBadRequest:    http.StatusBadRequest,
	apperror.CodeValidation:    http.StatusBadRequest,
	apperror.CodeAlreadyExists: http.StatusConflict,
	apperror.CodeConflict:      http.StatusConflict,
	apperror.CodeUnauthorized:  http.StatusUnauthorized,
	apperror.CodeNotFound:      http.StatusNotFound,
	apperror.CodeForbidden:     http.StatusForbidden,
	apperror.CodeInternal:      http.StatusInternalServerError,
}
