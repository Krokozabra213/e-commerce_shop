package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	eventsv1 "github.com/Krokozabra213/e-commerce_shop/api/gen/go/proto/events/v1"
	infrakafka "github.com/Krokozabra213/e-commerce_shop/infra/kafka"
	"github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/domain"
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

type EventPublisher struct {
	producer Producer
	logger   *slog.Logger
}

func NewEventPublisher(producer Producer, logger *slog.Logger) *EventPublisher {
	return &EventPublisher{
		producer: producer,
		logger:   logger,
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
	var jsonPayload map[string]interface{}
	if err := json.Unmarshal(event.Payload, &jsonPayload); err != nil {
		return fmt.Errorf("unmarshal json payload: %w", err)
	}

	protoEvent, err := mapJSONToInventoryReservedProto(event, jsonPayload)
	if err != nil {
		return fmt.Errorf("map to proto: %w", err)
	}

	value, err := proto.Marshal(protoEvent)
	if err != nil {
		return fmt.Errorf("marshal protobuf: %w", err)
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

	p.logger.Debug("publishing inventory.reserved event",
		slog.String("outbox_id", event.ID.String()),
		slog.String("order_id", event.AggregateID.String()),
		slog.String("topic", topic),
	)

	return p.producer.ProduceSync(ctx, topic, key, value, headers)
}

func (p *EventPublisher) publishInventoryReservationFailed(ctx context.Context, topic string, event *domain.OutboxEvent) error {
	var jsonPayload map[string]interface{}
	if err := json.Unmarshal(event.Payload, &jsonPayload); err != nil {
		return fmt.Errorf("unmarshal json payload: %w", err)
	}

	protoEvent, err := mapJSONToInventoryReservationFailedProto(event, jsonPayload)
	if err != nil {
		return fmt.Errorf("map to proto: %w", err)
	}

	p.logger.Debug("publishing inventory.reservation-failed event",
		slog.String("outbox_id", event.ID.String()),
		slog.String("order_id", event.AggregateID.String()),
		slog.String("topic", topic),
	)

	value, err := proto.Marshal(protoEvent)
	if err != nil {
		return fmt.Errorf("marshal protobuf: %w", err)
	}

	key := []byte(event.AggregateID.String())

	headers := map[string]string{
		"content-type":   "application/x-protobuf",
		"event-type":     event.EventType,
		"outbox-id":      event.ID.String(),
		"correlation-id": event.CorrelationID.String(),
		"aggregate-type": event.AggregateType,
		"aggregate-id":   event.AggregateID.String(),
		"failure-reason": "",
	}

	return p.producer.ProduceSync(ctx, topic, key, value, headers)
}

func mapJSONToInventoryReservedProto(
	event *domain.OutboxEvent,
	jsonPayload map[string]interface{},
) (*eventsv1.InventoryReservedEvent, error) {
	orderID, ok := jsonPayload["order_id"].(string)
	if !ok || orderID == "" {
		return nil, fmt.Errorf("missing or invalid order_id")
	}

	return &eventsv1.InventoryReservedEvent{
		Metadata: &eventsv1.EventMetadata{
			EventId:       event.ID.String(),
			EventType:     event.EventType,
			Timestamp:     timestamppb.New(event.CreatedAt),
			CorrelationId: event.CorrelationID.String(),
		},
		Reservation: &eventsv1.ReservationData{
			OrderId:    orderID,
			ReservedAt: timestamppb.New(event.CreatedAt),
		},
	}, nil
}

func mapJSONToInventoryReservationFailedProto(
	event *domain.OutboxEvent,
	jsonPayload map[string]interface{},
) (*eventsv1.InventoryReservationFailedEvent, error) {
	orderID, ok := jsonPayload["order_id"].(string)
	if !ok || orderID == "" {
		return nil, fmt.Errorf("missing or invalid order_id")
	}

	reasonStr, ok := jsonPayload["reason"].(string)
	if !ok || reasonStr == "" {
		return nil, fmt.Errorf("missing or invalid reason")
	}

	reasonEnum := mapReasonStringToProtoEnum(reasonStr)

	return &eventsv1.InventoryReservationFailedEvent{
		Metadata: &eventsv1.EventMetadata{
			EventId:       event.ID.String(),
			EventType:     event.EventType,
			Timestamp:     timestamppb.New(event.CreatedAt),
			CorrelationId: event.CorrelationID.String(),
		},
		Failure: &eventsv1.ReservationFailureData{
			OrderId:  orderID,
			Reason:   reasonEnum,
			Details:  reasonStr,
			FailedAt: timestamppb.New(event.CreatedAt),
		},
	}, nil
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
