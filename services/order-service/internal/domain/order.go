package domain

import (
	"time"

	"github.com/google/uuid"
)

type OrderStatus string

const (
	OrderStatusNew       OrderStatus = "NEW"
	OrderStatusReserved  OrderStatus = "RESERVED"
	OrderStatusPaid      OrderStatus = "PAID"
	OrderStatusShipped   OrderStatus = "SHIPPED"
	OrderStatusCompleted OrderStatus = "COMPLETED"
	OrderStatusCancelled OrderStatus = "CANCELLED"
)

func (o OrderStatus) String() string {
	return string(o)
}

type Order struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	IdempotencyKey uuid.UUID
	Status         OrderStatus
	TotalPrice     int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type OrderItem struct {
	ID        uuid.UUID
	OrderID   uuid.UUID
	ProductID string
	Quantity  int
	Price     int64
	CreatedAt time.Time
}

func NewOrderStateMachine() *StateMachine {
	return NewStateMachine().
		// Из NEW можно перейти в:
		AllowMultiple(
			string(OrderStatusNew),
			string(OrderStatusReserved),
			string(OrderStatusCancelled),
		).
		// Из RESERVED можно перейти в:
		AllowMultiple(
			string(OrderStatusReserved),
			string(OrderStatusPaid),
			string(OrderStatusCancelled),
		).
		// Из PAID можно перейти в:
		AllowMultiple(
			string(OrderStatusPaid),
			string(OrderStatusShipped),
		).
		// Из SHIPPED можно перейти в:
		Allow(
			string(OrderStatusShipped),
			string(OrderStatusCompleted),
		)
	// COMPLETED и CANCELLED - финальные состояния (нет переходов)
}
