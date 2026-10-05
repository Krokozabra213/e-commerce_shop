package health

import (
	"context"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/mongo"
)

type Handler struct {
	mongo *mongo.Client
}

func NewHandler(mongo *mongo.Client) *Handler {
	return &Handler{
		mongo: mongo,
	}
}

func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Get("/healthz", h.HealthCheck)
	router.Get("/readyz", h.ReadyCheck)
}

func (h *Handler) HealthCheck(c fiber.Ctx) error {
	return c.Status(http.StatusOK).JSON(fiber.Map{"status": "ready"})
}

func (h *Handler) ReadyCheck(c fiber.Ctx) error {
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
