package health

import (
	"context"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"
)

type Handler struct {
	redis redis.UniversalClient
}

func NewHandler(redis redis.UniversalClient) *Handler {
	return &Handler{
		redis: redis,
	}
}

func (h *Handler) HealthCheck(c fiber.Ctx) error {
	return c.SendStatus(fiber.StatusOK)
}

func (h *Handler) ReadyCheck(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()

	if err := h.redis.Ping(ctx).Err(); err != nil {
		return apperror.NewInternal("health.ReadyCheck.Redis", err, "Redis unavailable", nil)
	}

	return c.SendStatus(fiber.StatusOK)
}

func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Get("/healthz", h.HealthCheck)
	router.Get("/readyz", h.ReadyCheck)
}
