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
	pgPool                        Pinger
	inventoryReservedDLQ          Pinger
	inventoryReservFailedDLQ      Pinger
	inventoryReservedConsumer     Pinger
	inventoryReservFailedConsumer Pinger
	schemaRegistryClient          Pinger
}

func NewHealthHandler(
	pgPool Pinger,
	inventoryReservedDLQ Pinger,
	inventoryReservFailedDLQ Pinger,
	inventoryReservedConsumer Pinger,
	inventoryReservFailedConsumer Pinger,
	schemaRegistryClient Pinger,
) *HealthHandler {
	return &HealthHandler{
		schemaRegistryClient:          schemaRegistryClient,
		pgPool:                        pgPool,
		inventoryReservedDLQ:          inventoryReservedDLQ,
		inventoryReservFailedDLQ:      inventoryReservFailedDLQ,
		inventoryReservedConsumer:     inventoryReservedConsumer,
		inventoryReservFailedConsumer: inventoryReservFailedConsumer,
	}
}

func (h *HealthHandler) RegisterRoutes(router fiber.Router) {
	router.Get("/healthz", h.HealthCheck)
	router.Get("/readyz", h.ReadyCheck)
}

func (h *HealthHandler) HealthCheck(c fiber.Ctx) error {
	return c.Status(http.StatusOK).JSON(fiber.Map{"status": "ready"})
}

func (h *HealthHandler) ReadyCheck(c fiber.Ctx) error {

	ctx, cancel := context.WithTimeout(c.Context(), 3*time.Second)
	defer cancel()

	status := fiber.Map{"status": "ok"}
	statusCode := http.StatusOK

	if err := h.pgPool.Ping(ctx); err != nil {
		status["status"] = "error"
		status["postgres"] = "down"
		statusCode = http.StatusInternalServerError
	}

	if err := h.inventoryReservedDLQ.Ping(ctx); err != nil {
		status["status"] = "error"
		status["inventoryReservedDLQ"] = "down"
		statusCode = http.StatusInternalServerError
	}

	if err := h.inventoryReservFailedDLQ.Ping(ctx); err != nil {
		status["status"] = "error"
		status["inventoryReservFailedDLQ"] = "down"
		statusCode = http.StatusInternalServerError
	}

	if err := h.inventoryReservedConsumer.Ping(ctx); err != nil {
		status["status"] = "error"
		status["inventoryReservedConsumer"] = "down"
		statusCode = http.StatusInternalServerError
	}

	if err := h.inventoryReservFailedConsumer.Ping(ctx); err != nil {
		status["status"] = "error"
		status["inventoryReservFailedConsumer"] = "down"
		statusCode = http.StatusInternalServerError
	}

	if err := h.schemaRegistryClient.Ping(ctx); err != nil {
		status["status"] = "error"
		status["schemaRegistryClient"] = "down"
		statusCode = http.StatusInternalServerError
	}

	return c.Status(statusCode).JSON(status)
}
