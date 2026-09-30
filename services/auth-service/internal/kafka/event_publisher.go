package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	infrakafka "github.com/Krokozabra213/e-commerce_shop/infra/kafka"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/dto"
	"github.com/riferrei/srclient"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "github.com/Krokozabra213/e-commerce_shop/api/gen/go/proto/events/v1"
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
	RegisterOrGetSchema(topic string, protoSchemaText string) (*srclient.Schema, error)
}

type EventPublisher struct {
	producer       Producer
	schemaRegistry SchemaSerializer
	logger         *slog.Logger
}

func NewEventPublisher(producer Producer, schemaRegistry SchemaSerializer, logger *slog.Logger) *EventPublisher {
	return &EventPublisher{
		producer:       producer,
		schemaRegistry: schemaRegistry,
		logger:         logger,
	}
}

func (p *EventPublisher) Publish(ctx context.Context, event *domain.OutboxEvent) error {
	topic := event.EventType

	switch event.EventType {
	case infrakafka.TopicUserCreated:
		return p.publishUserCreated(ctx, topic, event)
	default:
		return fmt.Errorf("unknown event type: %s", event.EventType)
	}
}

func (p *EventPublisher) publishUserCreated(ctx context.Context, topic string, event *domain.OutboxEvent) error {
	user, err := mapPayloadToProtoUser(event.Payload)
	if err != nil {
		return fmt.Errorf("map payload to proto user: %w", err)
	}

	protoEvent := &eventsv1.UserCreated{
		User:      user,
		EventTime: timestamppb.Now(),
	}

	schema, err := p.schemaRegistry.RegisterOrGetSchema(topic, eventsv1.EventsProtoSchema)
	if err != nil {
		return fmt.Errorf("register proto schema: %w", err)
	}

	value, err := p.schemaRegistry.Serialize(ctx, schema.ID(), protoEvent)
	if err != nil {
		return fmt.Errorf("serialize with schema registry: %w", err)
	}

	headers := map[string]string{
		"content-type": "application/protobuf",
		"event-type":   event.EventType,
		"outbox-id":    event.ID.String(),
	}

	p.logger.Debug("publishing user.created event",
		slog.String("outbox_id", event.ID.String()),
		slog.String("user_id", user.Id),
	)

	return p.producer.ProduceSync(ctx, topic, []byte(user.Id), value, headers)
}

func mapPayloadToProtoUser(payload map[string]any) (*eventsv1.User, error) {
	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload map: %w", err)
	}

	var p dto.UserCreatedPayload
	if err := json.Unmarshal(bytes, &p); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}

	if p.ID == "" {
		return nil, fmt.Errorf("payload missing required field: id")
	}

	return &eventsv1.User{
		Id:        p.ID,
		Email:     p.Email,
		Roles:     p.Roles,
		CreatedAt: timestamppb.New(p.CreatedAt),
	}, nil
}
