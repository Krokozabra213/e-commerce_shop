package inframiddleware

import (
	"log/slog"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/httpx"
	log "github.com/Krokozabra213/e-commerce_shop/infra/logger"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/google/uuid"
)

func RequestLogger(logger *slog.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		requestID := c.Get(httpx.HeaderRequestID)

		if requestID == "" {
			requestID = requestid.FromContext(c)
		}

		if requestID == "" {
			requestID = uuid.New().String()
		}

		start := time.Now()

		loggerWithRequestID := logger.With("request_id", requestID) //nolint:revive

		ctx := c.Context()
		ctx = log.WithLogger(ctx, loggerWithRequestID)
		ctx = log.WithRequestID(ctx, requestID)
		c.SetContext(ctx)

		err := c.Next()

		loggerWithRequestID.InfoContext(c.Context(), "request",
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			slog.String("source_ip", c.IP()),
			slog.String("path", c.Path()),
			slog.String("method", c.Method()),
		)

		return err
	}
}
