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

type PaymentSucceededService interface {
	HandlePaymentSucceeded(ctx context.Context, input svcDTO.PaymentSucceededInput) error
}

type PaymentSucceededHandler struct {
	service        PaymentSucceededService
	schemaRegistry SchemaDeserializer
	logger         *slog.Logger
}

func NewPaymentSucceededHandler(
	service PaymentSucceededService,
	schemaRegistry SchemaDeserializer,
	logger *slog.Logger,
) *PaymentSucceededHandler {
	return &PaymentSucceededHandler{
		service:        service,
		schemaRegistry: schemaRegistry,
		logger:         logger,
	}
}

func (h *PaymentSucceededHandler) Handle(ctx context.Context, record *kgo.Record) error {
	var event eventsv1.PaymentSucceededEvent

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

	if event.Success == nil {
		return errors.New("битое сообщение: success == nil")
	}

	orderID, err := parseUUID(event.Success.OrderId)
	if err != nil {
		return fmt.Errorf("битое сообщение: invalid order_id: %w", err)
	}

	input := svcDTO.PaymentSucceededInput{
		EventID:       eventID,
		CorrelationID: correlationID,
		OrderID:       orderID,
	}

	if err := h.service.HandlePaymentSucceeded(ctx, input); err != nil {
		return fmt.Errorf("handle payment succeeded: %w", err)
	}

	h.logger.Debug("successfully processed payment.succeeded event",
		slog.String("order_id", orderID.String()),
		slog.String("correlation_id", correlationID.String()),
	)

	return nil
}
