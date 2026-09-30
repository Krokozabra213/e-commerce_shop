package kafkahandler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	eventsv1 "github.com/Krokozabra213/e-commerce_shop/api/gen/go/proto/events/v1"
	svcDTO "github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/service/dto"
	"github.com/twmb/franz-go/pkg/kgo"
)

type InventoryReservationFailedService interface {
	HandleInventoryReservationFailed(ctx context.Context, input svcDTO.InventoryReservationFailedInput) error
}

type InventoryReservationFailedHandler struct {
	service        InventoryReservationFailedService
	schemaRegistry SchemaDeserializer
	logger         *slog.Logger
}

func NewInventoryReservationFailedHandler(
	service InventoryReservationFailedService,
	schemaRegistry SchemaDeserializer,
	logger *slog.Logger,
) *InventoryReservationFailedHandler {
	return &InventoryReservationFailedHandler{
		service:        service,
		schemaRegistry: schemaRegistry,
		logger:         logger,
	}
}

func (h *InventoryReservationFailedHandler) Handle(ctx context.Context, record *kgo.Record) error {
	var event eventsv1.InventoryReservationFailedEvent

	if err := h.schemaRegistry.Deserialize(ctx, record.Value, &event); err != nil {
		return fmt.Errorf("битое сообщение: failed to deserialize: %w", err)
	}

	if event.Metadata == nil {
		return errors.New("битое сообщение: metadata == nil")
	}

	eventID, err := parseUUID(event.Metadata.EventId)
	if err != nil {
		return fmt.Errorf("битое сообщение: invalid event_id: %w", err)
	}

	correlationID, err := parseUUID(event.Metadata.CorrelationId)
	if err != nil {
		return fmt.Errorf("битое сообщение: invalid correlation_id: %w", err)
	}

	if event.Failure == nil {
		return errors.New("битое сообщение: failure == nil")
	}

	orderID, err := parseUUID(event.Failure.OrderId)
	if err != nil {
		return fmt.Errorf("битое сообщение: invalid order_id: %w", err)
	}

	reason := mapReservationFailureReason(event.Failure.Reason)

	input := svcDTO.InventoryReservationFailedInput{
		EventID:       eventID,
		CorrelationID: correlationID,
		OrderID:       orderID,
		Reason:        reason,
	}

	if err := h.service.HandleInventoryReservationFailed(ctx, input); err != nil {
		return fmt.Errorf("handle inventory reservation failed: %w", err)
	}

	h.logger.Debug("successfully processed inventory.reservation-failed event",
		slog.String("order_id", orderID.String()),
		slog.String("correlation_id", correlationID.String()),
		slog.String("reason", reason),
	)

	return nil
}

func mapReservationFailureReason(reason eventsv1.ReservationFailureReason) string {
	switch reason {
	case eventsv1.ReservationFailureReason_RESERVATION_FAILURE_REASON_INSUFFICIENT_STOCK:
		return "insufficient_stock"
	case eventsv1.ReservationFailureReason_RESERVATION_FAILURE_REASON_PRODUCT_NOT_FOUND:
		return "product_not_found"
	case eventsv1.ReservationFailureReason_RESERVATION_FAILURE_REASON_INTERNAL_ERROR:
		return "internal_error"
	default:
		return "unknown"
	}
}
