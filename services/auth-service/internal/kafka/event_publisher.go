package kafka

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	infrakafka "github.com/Krokozabra213/e-commerce_shop/infra/kafka"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
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

	value, err := proto.Marshal(protoEvent)
	if err != nil {
		return fmt.Errorf("marshal protobuf: %w", err)
	}

	headers := map[string]string{
		"content-type": "application/protobuf",
		"event-type":   event.EventType,
		"outbox-id":    event.ID.String(),
	}

	return p.producer.ProduceSync(ctx, topic, []byte(user.Id), value, headers)
}

func mapPayloadToProtoUser(payload map[string]interface{}) (*eventsv1.User, error) {
	id, _ := payload["id"].(string)
	if id == "" {
		return nil, fmt.Errorf("payload missing required field: id")
	}

	email, _ := payload["email"].(string)

	var roles []string
	if rawRoles, ok := payload["roles"].([]interface{}); ok {
		for _, r := range rawRoles {
			if s, ok := r.(string); ok {
				roles = append(roles, s)
			}
		}
	}

	var createdAt *timestamppb.Timestamp
	if rawTime, ok := payload["created_at"].(string); ok {
		t, err := time.Parse(time.RFC3339, rawTime)
		if err == nil {
			createdAt = timestamppb.New(t)
		}
	}

	return &eventsv1.User{
		Id:        id,
		Email:     email,
		Roles:     roles,
		CreatedAt: createdAt,
	}, nil
}
