package kafkahandler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	eventsv1 "github.com/Krokozabra213/e-commerce_shop/api/gen/go/proto/events/v1"
	svcDTO "github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/service/dto"
	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/protobuf/proto"
)

type SchemaDeserializer interface {
	Deserialize(ctx context.Context, payload []byte, dest proto.Message) error
}

type InventoryReservedService interface {
	HandleInventoryReserved(ctx context.Context, input *svcDTO.InventoryReservedInput) error
}

type InventoryReservedHandler struct {
	service        InventoryReservedService
	schemaRegistry SchemaDeserializer
	logger         *slog.Logger
}

func NewInventoryReservedHandler(
	service InventoryReservedService,
	schemaRegistry SchemaDeserializer,
	logger *slog.Logger,
) *InventoryReservedHandler {
	return &InventoryReservedHandler{
		service:        service,
		schemaRegistry: schemaRegistry,
		logger:         logger,
	}
}

func (h *InventoryReservedHandler) Handle(ctx context.Context, record *kgo.Record) error {
	var event eventsv1.InventoryReservedEvent

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

	if event.Reservation == nil {
		return errors.New("битое сообщение: reservation == nil")
	}

	orderID, err := parseUUID(event.Reservation.OrderId)
	if err != nil {
		return fmt.Errorf("битое сообщение: invalid order_id: %w", err)
	}

	input := svcDTO.InventoryReservedInput{
		EventID:       eventID,
		CorrelationID: correlationID,
		OrderID:       orderID,
	}

	if err := h.service.HandleInventoryReserved(ctx, &input); err != nil {
		return fmt.Errorf("handle inventory reserved: %w", err)
	}

	h.logger.Debug("successfully processed inventory.reserved event",
		slog.String("order_id", orderID.String()),
		slog.String("correlation_id", correlationID.String()),
		slog.String("event_id", eventID.String()),
	)

	return nil
}
