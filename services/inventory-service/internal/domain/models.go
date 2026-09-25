package domain

import (
	"time"

	"github.com/google/uuid"
)

type Stock struct {
	ProductID         string
	AvailableQuantity int
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type ReservationStatus string

const (
	ReservationStatusActive   ReservationStatus = "ACTIVE"
	ReservationStatusReleased ReservationStatus = "RELEASED"
)

type Reservation struct {
	ID         uuid.UUID
	OrderID    uuid.UUID
	ProductID  string
	Quantity   int
	Status     ReservationStatus
	CreatedAt  time.Time
	ReleasedAt *time.Time
}

type OutboxEvent struct {
	ID            uuid.UUID
	CorrelationID uuid.UUID
	AggregateType string
	AggregateID   uuid.UUID
	EventType     string
	Payload       []byte
	CreatedAt     time.Time
	PublishedAt   *time.Time
}
