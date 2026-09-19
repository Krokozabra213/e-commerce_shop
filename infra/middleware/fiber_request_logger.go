package inframiddleware

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/google/uuid"
)

const (
	LoggerKey = "logger"
)

func RequestLogger(logger *slog.Logger) func(c fiber.Ctx) error {
	return func(c fiber.Ctx) error {
		requestId := requestid.FromContext(c)
		if requestId == "" {
			requestId = uuid.New().String()
		}

		start := time.Now()

		loggerWithRequestId := logger.With("request_id", requestId)

		c.Locals(LoggerKey, loggerWithRequestId)

		err := c.Next()

		loggerWithRequestId.Info("request",
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			slog.String("source_ip", c.IP()),
			slog.String("path", c.Path()),
			slog.String("method", c.Method()),
		)

		return err
	}
}

func GetLogger(c fiber.Ctx) *slog.Logger {
	logger := fiber.Locals[*slog.Logger](c, LoggerKey)
	if logger == nil {
		return slog.Default()
	}

	return logger
}
