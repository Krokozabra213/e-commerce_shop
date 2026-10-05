package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	eventsv1 "github.com/Krokozabra213/e-commerce_shop/api/gen/go/proto/events/v1"
	infrakafka "github.com/Krokozabra213/e-commerce_shop/infra/kafka"
	"github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/domain"
	"github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/dto"
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

type EventPublisher struct {
	producer       Producer
	schemaRegistry SchemaSerializer
	logger         *slog.Logger
}

func NewEventPublisher(
	producer Producer,
	schemaRegistry SchemaSerializer,
	logger *slog.Logger,
) *EventPublisher {
	return &EventPublisher{
		producer:       producer,
		schemaRegistry: schemaRegistry,
		logger:         logger,
	}
}

func (p *EventPublisher) Publish(ctx context.Context, event *domain.OutboxEvent) error {
	topic := event.EventType

	switch event.EventType {
	case infrakafka.TopicInventoryReserved:
		return p.publishInventoryReserved(ctx, topic, event)
	case infrakafka.TopicInventoryReservFailed:
		return p.publishInventoryReservationFailed(ctx, topic, event)
	default:
		return fmt.Errorf("unknown event type: %s", event.EventType)
	}
}

func (p *EventPublisher) publishInventoryReserved(ctx context.Context, topic string, event *domain.OutboxEvent) error {
	var payload dto.InventoryReservedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal inventory reserved payload: %w", err)
	}

	if payload.OrderID == "" {
		return fmt.Errorf("payload missing required field: order_id")
	}

	protoEvent := &eventsv1.InventoryReservedEvent{
		Metadata: buildMetadata(event),
		Reservation: &eventsv1.ReservationData{
			OrderId:    payload.OrderID,
			ReservedAt: timestamppb.New(event.CreatedAt),
		},
	}

	p.logger.Debug("publishing inventory.reserved event",
		slog.String("outbox_id", event.ID.String()),
		slog.String("order_id", event.AggregateID.String()),
		slog.String("topic", topic),
	)

	return p.produceEvent(ctx, topic, protoEvent, event, nil)
}

func (p *EventPublisher) publishInventoryReservationFailed(ctx context.Context, topic string, event *domain.OutboxEvent) error {
	var payload dto.InventoryReservationFailedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal inventory failed payload: %w", err)
	}

	if payload.OrderID == "" {
		return fmt.Errorf("payload missing required field: order_id")
	}
	if payload.Reason == "" {
		return fmt.Errorf("payload missing required field: reason")
	}

	reasonEnum := mapReasonStringToProtoEnum(payload.Reason)

	protoEvent := &eventsv1.InventoryReservationFailedEvent{
		Metadata: buildMetadata(event),
		Failure: &eventsv1.ReservationFailureData{
			OrderId:  payload.OrderID,
			Reason:   reasonEnum,
			Details:  payload.Reason,
			FailedAt: timestamppb.New(event.CreatedAt),
		},
	}

	p.logger.Debug("publishing inventory.reservation-failed event",
		slog.String("outbox_id", event.ID.String()),
		slog.String("order_id", event.AggregateID.String()),
		slog.String("topic", topic),
	)

	extraHeaders := map[string]string{
		"failure-reason": payload.Reason,
	}

	return p.produceEvent(ctx, topic, protoEvent, event, extraHeaders)
}

func buildMetadata(event *domain.OutboxEvent) *eventsv1.EventMetadata {
	return &eventsv1.EventMetadata{
		EventId:       event.ID.String(),
		EventType:     event.EventType,
		Timestamp:     timestamppb.New(event.CreatedAt),
		CorrelationId: event.CorrelationID.String(),
	}
}

func (p *EventPublisher) produceEvent(
	ctx context.Context,
	topic string,
	protoEvent proto.Message,
	event *domain.OutboxEvent,
	extraHeaders map[string]string,
) error {
	schema, err := p.schemaRegistry.RegisterOrGetSchema(topic, eventsv1.EventsProtoSchema)
	if err != nil {
		return fmt.Errorf("register proto schema: %w", err)
	}

	value, err := p.schemaRegistry.Serialize(ctx, schema.ID(), protoEvent)
	if err != nil {
		return fmt.Errorf("serialize with schema registry: %w", err)
	}

	key := []byte(event.AggregateID.String())

	headers := map[string]string{
		"content-type":   "application/x-protobuf",
		"event-type":     event.EventType,
		"outbox-id":      event.ID.String(),
		"correlation-id": event.CorrelationID.String(),
		"aggregate-type": event.AggregateType,
		"aggregate-id":   event.AggregateID.String(),
	}

	for k, v := range extraHeaders {
		headers[k] = v
	}

	return p.producer.ProduceSync(ctx, topic, key, value, headers)
}

func mapReasonStringToProtoEnum(reason string) eventsv1.ReservationFailureReason {
	switch reason {
	case domain.ReservationFailedReasonInsufficientStock.String():
		return eventsv1.ReservationFailureReason_RESERVATION_FAILURE_REASON_INSUFFICIENT_STOCK
	case domain.ReservationFailedReasonNotFound.String():
		return eventsv1.ReservationFailureReason_RESERVATION_FAILURE_REASON_PRODUCT_NOT_FOUND
	case domain.ReservationFailedReasonInternal.String():
		return eventsv1.ReservationFailureReason_RESERVATION_FAILURE_REASON_INTERNAL_ERROR
	default:
		return eventsv1.ReservationFailureReason_RESERVATION_FAILURE_REASON_UNSPECIFIED
	}
}
