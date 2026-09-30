package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	eventsv1 "github.com/Krokozabra213/e-commerce_shop/api/gen/go/proto/events/v1"
	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/protobuf/proto"
)

type UserCreator interface {
	CreateFromEvent(
		ctx context.Context,
		userID uuid.UUID,
		email string,
		roles []string,
		createdAt time.Time,
	) error
}

type SchemaDeserializer interface {
	Deserialize(ctx context.Context, payload []byte, dest proto.Message) error
}

type UserCreatedHandler struct {
	service        UserCreator
	schemaRegistry SchemaDeserializer
	logger         *slog.Logger
}

func NewUserCreatedHandler(service UserCreator, schemaRegistry SchemaDeserializer, logger *slog.Logger) *UserCreatedHandler {
	return &UserCreatedHandler{
		service:        service,
		logger:         logger,
		schemaRegistry: schemaRegistry,
	}
}

func (h *UserCreatedHandler) Handle(ctx context.Context, record *kgo.Record) error {
	var event eventsv1.UserCreated

	if err := h.schemaRegistry.Deserialize(ctx, record.Value, &event); err != nil {
		return fmt.Errorf("битое сообщение: failed to deserialize with schema registry: %w", err)
	}

	if event.User == nil {
		return errors.New("битое сообщение, user==nil")
	}

	userID, err := uuid.Parse(event.User.GetId())
	if err != nil {
		return errors.New("битое сообщение, user_id не uuid")
	}

	email := event.User.GetEmail()
	if email == "" {
		return errors.New("битое сообщение, пустой email")
	}

	roles := event.User.GetRoles()

	var createdAt time.Time
	if event.User.GetCreatedAt() != nil {
		createdAt = event.User.GetCreatedAt().AsTime()
	} else {
		createdAt = time.Now().UTC()
	}

	if err := h.service.CreateFromEvent(ctx, userID, email, roles, createdAt); err != nil {
		return fmt.Errorf("create user from event: %w", err)
	}

	return nil
}
