package health

import (
	"context"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

type HealthHandler struct {
	pgPool Pinger
	broker Pinger
	redis  redis.UniversalClient
}

func NewHealthHandler(pgPool Pinger, broker Pinger, redis redis.UniversalClient) *HealthHandler {
	return &HealthHandler{
		pgPool: pgPool,
		redis:  redis,
		broker: broker,
	}
}

func (h *HealthHandler) RegisterRoutes(router fiber.Router) {
	router.Get("/healthz", h.HealthCheck)
	router.Get("/readyz", h.ReadyCheck)
}

func (h *HealthHandler) HealthCheck(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 3*time.Second)
	defer cancel()

	status := fiber.Map{"status": "ok"}
	statusCode := http.StatusOK

	if err := h.pgPool.Ping(ctx); err != nil {
		status["status"] = "error"
		status["postgres"] = "down"
		statusCode = http.StatusInternalServerError
	}

	if err := h.redis.Ping(ctx).Err(); err != nil {
		status["status"] = "error"
		status["redis"] = "down"
		c.Status(http.StatusInternalServerError)
	}

	if err := h.broker.Ping(ctx); err != nil {
		status["status"] = "error"
		status["broker"] = "down"
		statusCode = http.StatusInternalServerError
	}

	return c.Status(statusCode).JSON(status)
}

func (h *HealthHandler) ReadyCheck(c fiber.Ctx) error {
	return c.Status(http.StatusOK).JSON(fiber.Map{"status": "ready"})
}
