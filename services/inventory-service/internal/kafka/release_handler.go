package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	eventsv1 "github.com/Krokozabra213/e-commerce_shop/api/gen/go/proto/events/v1"
	"github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/service"
	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/protobuf/proto"
)

type ReservationReleaser interface {
	Release(ctx context.Context, input service.ReleaseInput) error
}

type OrderCancelledHandler struct {
	service ReservationReleaser
	logger  *slog.Logger
}

func NewOrderCancelledHandler(service ReservationReleaser, logger *slog.Logger) *OrderCancelledHandler {
	return &OrderCancelledHandler{
		service: service,
		logger:  logger,
	}
}

func (h *OrderCancelledHandler) Handle(ctx context.Context, record *kgo.Record) error {
	var event eventsv1.OrderCancelledEvent
	if err := proto.Unmarshal(record.Value, &event); err != nil {
		return errors.New("битое сообщение: failed to unmarshal proto")
	}

	if event.Metadata == nil {
		return errors.New("битое сообщение: metadata == nil")
	}

	correlationID, err := parseUUID(event.Metadata.CorrelationId)
	if err != nil {
		return fmt.Errorf("битое сообщение: invalid correlation_id: %w", err)
	}

	if event.Cancellation == nil {
		return errors.New("битое сообщение: cancellation == nil")
	}

	orderID, err := parseUUID(event.Cancellation.OrderId)
	if err != nil {
		return fmt.Errorf("битое сообщение: invalid order_id: %w", err)
	}

	input := service.ReleaseInput{
		CorrelationID: correlationID,
		OrderID:       orderID,
	}

	if err := h.service.Release(ctx, input); err != nil {
		return fmt.Errorf("release inventory: %w", err)
	}

	h.logger.Info("successfully released inventory for cancelled order",
		slog.String("order_id", orderID.String()),
		slog.String("correlation_id", correlationID.String()),
		slog.String("reason", event.Cancellation.Reason),
	)

	return nil
}
