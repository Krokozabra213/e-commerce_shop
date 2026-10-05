package kafkapublisher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	eventsv1 "github.com/Krokozabra213/e-commerce_shop/api/gen/go/proto/events/v1"
	infrakafka "github.com/Krokozabra213/e-commerce_shop/infra/kafka"
	"github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/domain"
	svcDTO "github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/service/dto"
	"github.com/riferrei/srclient"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Producer interface {
	ProduceSync(
		ctx context.Context,
		topic string,
		key, value []byte,
		headers map[string]string,
	) error
}

type SchemaSerializer interface {
	Serialize(ctx context.Context, schemaID int, message proto.Message) ([]byte, error)
	RegisterOrGetSchema(topic, protoSchemaText string) (*srclient.Schema, error)
}

type OrderEventPublisher struct {
	producer       Producer
	schemaRegistry SchemaSerializer
	logger         *slog.Logger
}

func NewOrderEventPublisher(
	producer Producer,
	schemaRegistry SchemaSerializer,
	logger *slog.Logger,
) *OrderEventPublisher {
	return &OrderEventPublisher{
		producer:       producer,
		schemaRegistry: schemaRegistry,
		logger:         logger,
	}
}

func (p *OrderEventPublisher) Publish(ctx context.Context, event *domain.OutboxEvent) error {
	topic := event.EventType

	switch event.EventType {
	case infrakafka.TopicOrderCreated:
		return p.publishOrderCreated(ctx, topic, event)
	case infrakafka.TopicOrderCreatePayment:
		return p.publishPaymentChargeRequest(ctx, topic, event)
	case infrakafka.TopicOrderCancelInventory:
		return p.publishOrderCancelled(ctx, topic, event)
	default:
		return fmt.Errorf("unknown event type: %s", event.EventType)
	}
}

func (p *OrderEventPublisher) publishOrderCreated(ctx context.Context, topic string, event *domain.OutboxEvent) error {
	orderData, err := mapPayloadToOrderData(event.Payload)
	if err != nil {
		return fmt.Errorf("unmarshal order data: %w", err)
	}

	protoEvent := &eventsv1.OrderCreatedEvent{
		Metadata: buildMetadata(event),
		Order:    orderData,
	}

	return p.produceEvent(ctx, topic, []byte(orderData.Id), protoEvent, event)
}

func (p *OrderEventPublisher) publishPaymentChargeRequest(ctx context.Context, topic string, event *domain.OutboxEvent) error {
	chargeData, err := mapPayloadToPaymentChargeData(event.Payload)
	if err != nil {
		return fmt.Errorf("unmarshal payment charge data: %w", err)
	}

	protoEvent := &eventsv1.PaymentChargeRequestEvent{
		Metadata: buildMetadata(event),
		Charge:   chargeData,
	}

	return p.produceEvent(ctx, topic, []byte(chargeData.OrderId), protoEvent, event)
}

func (p *OrderEventPublisher) publishOrderCancelled(ctx context.Context, topic string, event *domain.OutboxEvent) error {
	cancellationData, err := mapPayloadToOrderCancellationData(event.Payload)
	if err != nil {
		return fmt.Errorf("unmarshal order cancellation data: %w", err)
	}

	protoEvent := &eventsv1.OrderCancelledEvent{
		Metadata:     buildMetadata(event),
		Cancellation: cancellationData,
	}

	return p.produceEvent(ctx, topic, []byte(cancellationData.OrderId), protoEvent, event)
}

func buildMetadata(event *domain.OutboxEvent) *eventsv1.EventMetadata {
	return &eventsv1.EventMetadata{
		EventId:       event.ID.String(),
		EventType:     event.EventType,
		Timestamp:     timestamppb.New(event.CreatedAt),
		CorrelationId: event.CorrelationID.String(),
	}
}

func (p *OrderEventPublisher) produceEvent(
	ctx context.Context,
	topic string,
	key []byte,
	protoEvent proto.Message,
	event *domain.OutboxEvent,
) error {
	schema, err := p.schemaRegistry.RegisterOrGetSchema(topic, eventsv1.EventsProtoSchema)
	if err != nil {
		return fmt.Errorf("register proto schema: %w", err)
	}

	value, err := p.schemaRegistry.Serialize(ctx, schema.ID(), protoEvent)
	if err != nil {
		return fmt.Errorf("serialize with schema registry: %w", err)
	}

	headers := map[string]string{
		"content-type":   "application/protobuf",
		"event-type":     event.EventType,
		"outbox-id":      event.ID.String(),
		"correlation-id": event.CorrelationID.String(),
	}

	return p.producer.ProduceSync(ctx, topic, key, value, headers)
}

func mapPayloadToOrderData(payload map[string]any) (*eventsv1.OrderData, error) {
	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload map: %w", err)
	}

	var p svcDTO.OrderCreatedPayload
	if err := json.Unmarshal(bytes, &p); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}

	if p.OrderID == "" {
		return nil, fmt.Errorf("payload missing required field: order_id")
	}

	items := make([]*eventsv1.ProductItem, len(p.Items))
	for i, item := range p.Items {
		items[i] = &eventsv1.ProductItem{
			ProductId: item.ProductID,
			Quantity:  item.Quantity,
		}
	}

	return &eventsv1.OrderData{
		Id:        p.OrderID,
		Items:     items,
		CreatedAt: timestamppb.New(p.CreatedAt),
	}, nil
}

func mapPayloadToPaymentChargeData(payload map[string]any) (*eventsv1.PaymentChargeData, error) {
	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload map: %w", err)
	}

	var p svcDTO.PaymentChargePayload
	if err := json.Unmarshal(bytes, &p); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}

	if p.OrderID == "" {
		return nil, fmt.Errorf("payload missing required field: order_id")
	}
	if p.UserID == "" {
		return nil, fmt.Errorf("payload missing required field: user_id")
	}

	return &eventsv1.PaymentChargeData{
		OrderId:     p.OrderID,
		UserId:      p.UserID,
		Amount:      p.Amount,
		RequestedAt: timestamppb.New(p.RequestedAt),
	}, nil
}

func mapPayloadToOrderCancellationData(payload map[string]any) (*eventsv1.OrderCancellationData, error) {
	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload map: %w", err)
	}

	var p svcDTO.OrderCancellationPayload
	if err := json.Unmarshal(bytes, &p); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}

	if p.OrderID == "" {
		return nil, fmt.Errorf("payload missing required field: order_id")
	}

	return &eventsv1.OrderCancellationData{
		OrderId:     p.OrderID,
		Reason:      p.Reason,
		CancelledAt: timestamppb.New(p.CancelledAt),
	}, nil
}
