package health

import (
	"context"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/mongo"
)

type HealthHandler struct {
	mongo *mongo.Client
}

func NewHealthHandler(mongo *mongo.Client) *HealthHandler {
	return &HealthHandler{
		mongo: mongo,
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

	if err := h.mongo.Ping(ctx, nil); err != nil {
		status["status"] = "error"
		status["mongo"] = "down"
		statusCode = http.StatusInternalServerError
	}

	return c.Status(statusCode).JSON(status)
}

func (h *HealthHandler) ReadyCheck(c fiber.Ctx) error {
	return c.Status(http.StatusOK).JSON(fiber.Map{"status": "ready"})
}
