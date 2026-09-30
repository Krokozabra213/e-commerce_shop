package svcDTO

import "github.com/google/uuid"

type OrderItemInput struct {
	ProductID string
	Quantity  int
}

type CreateOrderInput struct {
	UserID         uuid.UUID
	IdempotencyKey uuid.UUID
	Items          []OrderItemInput
}

type CreateOrderOutput struct {
	OrderID    uuid.UUID
	Status     string
	TotalPrice int64
}

type InventoryReservedInput struct {
	EventID       uuid.UUID
	CorrelationID uuid.UUID
	OrderID       uuid.UUID
}

type InventoryReservationFailedInput struct {
	EventID       uuid.UUID
	CorrelationID uuid.UUID
	OrderID       uuid.UUID
	Reason        string
}

type PaymentSucceededInput struct {
	EventID       uuid.UUID
	CorrelationID uuid.UUID
	OrderID       uuid.UUID
}

type PaymentFailedInput struct {
	EventID       uuid.UUID
	CorrelationID uuid.UUID
	OrderID       uuid.UUID
	Reason        string
}
