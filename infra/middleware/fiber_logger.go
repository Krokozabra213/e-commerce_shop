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
		requestId := c.Get(httpx.HeaderRequestID)

		if requestId == "" {
			requestId = requestid.FromContext(c)
		}

		if requestId == "" {
			requestId = uuid.New().String()
		}

		start := time.Now()

		loggerWithRequestId := logger.With("request_id", requestId)

		ctx := c.Context()
		ctx = log.WithLogger(ctx, loggerWithRequestId)
		ctx = log.WithRequestID(ctx, requestId)
		c.SetContext(ctx)

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
