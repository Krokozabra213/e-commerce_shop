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

type Handler struct {
	pgPool                 Pinger
	orderCancelledDLQ      Pinger
	orderCreatedDLQ        Pinger
	orderCreatedConsumer   Pinger
	orderCancelledConsumer Pinger
	schemaRegistryClient   Pinger
}

func NewHandler(
	pgPool Pinger,
	orderCancelledDLQ Pinger,
	orderCreatedDLQ Pinger,
	orderCreatedConsumer Pinger,
	orderCancelledConsumer Pinger,
	schemaRegistryClient Pinger,
) *Handler {
	return &Handler{
		schemaRegistryClient:   schemaRegistryClient,
		pgPool:                 pgPool,
		orderCancelledDLQ:      orderCancelledDLQ,
		orderCreatedDLQ:        orderCreatedDLQ,
		orderCreatedConsumer:   orderCreatedConsumer,
		orderCancelledConsumer: orderCancelledConsumer,
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

	if err := h.schemaRegistryClient.Ping(ctx); err != nil {
		status["status"] = "error"
		status["schemaRegistryClient"] = "down"
		statusCode = http.StatusInternalServerError
	}

	return c.Status(statusCode).JSON(status)
}
