package domain

import (
	"time"

	"github.com/google/uuid"
)

type OutboxEvent struct {
	ID            uuid.UUID
	AggregateType string
	AggregateID   uuid.UUID
	EventType     string
	Payload       map[string]any
	CreatedAt     time.Time
	PublishedAt   *time.Time
}
