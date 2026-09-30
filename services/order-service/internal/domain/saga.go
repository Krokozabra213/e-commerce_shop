package domain

import (
	"time"

	"github.com/google/uuid"
)

type SagaStep string

const (
	SagaStepReservingInventory    SagaStep = "RESERVING_INVENTORY"
	SagaStepChargingPayment       SagaStep = "CHARGING_PAYMENT"
	SagaStepCompensatingPayment   SagaStep = "COMPENSATING_PAYMENT"
	SagaStepCompensatingInventory SagaStep = "COMPENSATING_INVENTORY"
	SagaStepCompleted             SagaStep = "COMPLETED"
)

func (s SagaStep) String() string {
	return string(s)
}

type SagaStatus string

const (
	SagaStatusInProgress   SagaStatus = "IN_PROGRESS"
	SagaStatusCompleted    SagaStatus = "COMPLETED"
	SagaStatusFailed       SagaStatus = "FAILED"
	SagaStatusCompensating SagaStatus = "COMPENSATING"
	SagaStatusCompensated  SagaStatus = "COMPENSATED"
)

func (s SagaStatus) String() string {
	return string(s)
}

type SagaState struct {
	OrderID       uuid.UUID
	CorrelationID uuid.UUID
	CurrentStep   SagaStep
	Status        SagaStatus
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type OutboxEvent struct {
	ID            uuid.UUID
	CorrelationID uuid.UUID
	AggregateType string
	AggregateID   uuid.UUID
	EventType     string
	Payload       map[string]any
	CreatedAt     time.Time
	PublishedAt   *time.Time
}

type InboxEvent struct {
	ID            uuid.UUID
	EventID       uuid.UUID
	CorrelationID uuid.UUID
	EventType     string
	Payload       map[string]any
	ProcessedAt   time.Time
}

func NewSagaStepStateMachine() *StateMachine {
	sm := NewStateMachine()

	sm.
		// Из RESERVING_INVENTORY можно перейти в:
		AllowMultiple(
			string(SagaStepReservingInventory),
			string(SagaStepChargingPayment), // успех
			string(SagaStepCompleted),       // провал (нечего компенсировать)
		).
		// Из CHARGING_PAYMENT можно перейти в:
		AllowMultiple(
			string(SagaStepChargingPayment),
			string(SagaStepCompleted),             // успех
			string(SagaStepCompensatingInventory), // провал (нужна компенсация)
		)

	// Compensation Path (COMPENSATING)
	sm.
		// Из COMPENSATING_PAYMENT можно перейти в:
		Allow(
			string(SagaStepCompensatingPayment),
			string(SagaStepCompensatingInventory),
		).
		// Из COMPENSATING_INVENTORY можно перейти в:
		Allow(
			string(SagaStepCompensatingInventory),
			string(SagaStepCompleted),
		)

	return sm
}

func NewSagaStatusStateMachine() *StateMachine {
	return NewStateMachine().
		// Из IN_PROGRESS можно перейти в:
		AllowMultiple(
			string(SagaStatusInProgress),
			string(SagaStatusCompleted),
			string(SagaStatusFailed),
			string(SagaStatusCompensating),
		).
		// Из COMPENSATING можно перейти в:
		Allow(
			string(SagaStatusCompensating),
			string(SagaStatusCompensated),
		)
	// COMPLETED, FAILED, COMPENSATED - финальные состояния
}
