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

type PaymentFailedService interface {
	HandlePaymentFailed(ctx context.Context, input svcDTO.PaymentFailedInput) error
}

type PaymentFailedHandler struct {
	service        PaymentFailedService
	schemaRegistry SchemaDeserializer
	logger         *slog.Logger
}

func NewPaymentFailedHandler(
	service PaymentFailedService,
	schemaRegistry SchemaDeserializer,
	logger *slog.Logger,
) *PaymentFailedHandler {
	return &PaymentFailedHandler{
		service:        service,
		schemaRegistry: schemaRegistry,
		logger:         logger,
	}
}

func (h *PaymentFailedHandler) Handle(ctx context.Context, record *kgo.Record) error {
	var event eventsv1.PaymentFailedEvent

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

	reason := mapPaymentFailureReason(event.Failure.Reason)

	input := svcDTO.PaymentFailedInput{
		EventID:       eventID,
		CorrelationID: correlationID,
		OrderID:       orderID,
		Reason:        reason,
	}

	if err := h.service.HandlePaymentFailed(ctx, input); err != nil {
		return fmt.Errorf("handle payment failed: %w", err)
	}

	h.logger.Debug("successfully processed payment.failed event",
		slog.String("order_id", orderID.String()),
		slog.String("correlation_id", correlationID.String()),
		slog.String("reason", reason),
	)

	return nil
}

func mapPaymentFailureReason(reason eventsv1.PaymentFailureReason) string {
	switch reason {
	case eventsv1.PaymentFailureReason_PAYMENT_FAILURE_REASON_INSUFFICIENT_FUNDS:
		return "insufficient_funds"
	case eventsv1.PaymentFailureReason_PAYMENT_FAILURE_REASON_CARD_DECLINED:
		return "card_declined"
	case eventsv1.PaymentFailureReason_PAYMENT_FAILURE_REASON_INVALID_PAYMENT_METHOD:
		return "invalid_payment_method"
	case eventsv1.PaymentFailureReason_PAYMENT_FAILURE_REASON_INTERNAL_ERROR:
		return "internal_error"
	default:
		return "unknown"
	}
}
