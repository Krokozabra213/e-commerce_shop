package health

import (
	"context"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v3"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

type HealthHandler struct {
	pgPool                 Pinger
	orderCancelledDLQ      Pinger
	orderCreatedDLQ        Pinger
	orderCreatedConsumer   Pinger
	orderCancelledConsumer Pinger
}

func NewHealthHandler(
	pgPool Pinger,
	orderCancelledDLQ Pinger,
	orderCreatedDLQ Pinger,
	orderCreatedConsumer Pinger,
	orderCancelledConsumer Pinger,
) *HealthHandler {
	return &HealthHandler{
		pgPool:                 pgPool,
		orderCancelledDLQ:      orderCancelledDLQ,
		orderCreatedDLQ:        orderCreatedDLQ,
		orderCreatedConsumer:   orderCreatedConsumer,
		orderCancelledConsumer: orderCancelledConsumer,
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

	if err := h.orderCancelledDLQ.Ping(ctx); err != nil {
		status["status"] = "error"
		status["orderCancelledDLQ"] = "down"
		statusCode = http.StatusInternalServerError
	}

	if err := h.orderCreatedDLQ.Ping(ctx); err != nil {
		status["status"] = "error"
		status["orderCreatedDLQ"] = "down"
		statusCode = http.StatusInternalServerError
	}

	if err := h.orderCreatedConsumer.Ping(ctx); err != nil {
		status["status"] = "error"
		status["orderCreatedConsumer"] = "down"
		statusCode = http.StatusInternalServerError
	}

	if err := h.orderCancelledConsumer.Ping(ctx); err != nil {
		status["status"] = "error"
		status["orderCancelledConsumer"] = "down"
		statusCode = http.StatusInternalServerError
	}

	return c.Status(statusCode).JSON(status)
}

func (h *HealthHandler) ReadyCheck(c fiber.Ctx) error {
	return c.Status(http.StatusOK).JSON(fiber.Map{"status": "ready"})
}
