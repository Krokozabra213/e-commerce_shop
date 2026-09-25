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

type ReservationReserver interface {
	Reserve(ctx context.Context, input service.ReserveInput) error
}

type OrderCreatedHandler struct {
	service ReservationReserver
	logger  *slog.Logger
}

func NewOrderCreatedHandler(service ReservationReserver, logger *slog.Logger) *OrderCreatedHandler {
	return &OrderCreatedHandler{
		service: service,
		logger:  logger,
	}
}

func (h *OrderCreatedHandler) Handle(ctx context.Context, record *kgo.Record) error {
	var event eventsv1.OrderCreatedEvent
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

	if event.Order == nil {
		return errors.New("битое сообщение: order == nil")
	}

	orderID, err := parseUUID(event.Order.Id)
	if err != nil {
		return fmt.Errorf("битое сообщение: invalid order_id: %w", err)
	}

	if len(event.Order.Items) == 0 {
		return errors.New("битое сообщение: empty items")
	}

	items, err := h.validateAndConvertItems(event.Order.Items)
	if err != nil {
		return fmt.Errorf("битое сообщение: %w", err)
	}

	input := service.ReserveInput{
		CorrelationID: correlationID,
		OrderID:       orderID,
		Items:         items,
	}

	if err := h.service.Reserve(ctx, input); err != nil {
		return fmt.Errorf("reserve inventory: %w", err)
	}

	h.logger.Info("successfully reserved inventory for order",
		slog.String("order_id", orderID.String()),
		slog.String("correlation_id", correlationID.String()),
		slog.Int("items_count", len(items)),
	)

	return nil
}

func (h *OrderCreatedHandler) validateAndConvertItems(protoItems []*eventsv1.ProductItem) ([]service.ProductItem, error) {
	seenProducts := make(map[string]int)

	for i, item := range protoItems {
		if item == nil {
			return nil, fmt.Errorf("nil item at index %d", i)
		}

		if item.ProductId == "" {
			return nil, fmt.Errorf("empty product_id at index %d", i)
		}

		if item.Quantity <= 0 {
			return nil, fmt.Errorf("invalid quantity %d for product %s at index %d",
				item.Quantity, item.ProductId, i)
		}

		if existingQty, exists := seenProducts[item.ProductId]; exists {
			h.logger.Warn("duplicate product_id in order items, merging quantities",
				slog.String("product_id", item.ProductId),
				slog.Int("existing_quantity", existingQty),
				slog.Int("new_quantity", int(item.Quantity)),
				slog.Int("total_quantity", existingQty+int(item.Quantity)),
			)
			seenProducts[item.ProductId] = existingQty + int(item.Quantity)
		} else {
			seenProducts[item.ProductId] = int(item.Quantity)
		}
	}

	items := make([]service.ProductItem, 0, len(seenProducts))
	for productID, quantity := range seenProducts {
		items = append(items, service.ProductItem{
			ProductID: productID,
			Quantity:  quantity,
		})
	}

	return items, nil
}
