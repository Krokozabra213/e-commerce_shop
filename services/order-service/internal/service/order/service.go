package orderService

import (
	"context"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/domain"
	"github.com/google/uuid"
)

type TxManager interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

type ProductServiceClient interface {
	GetPrices(ctx context.Context, id []string) (prices map[string]int64, err error)
}

type OrderRepository interface {
	Create(ctx context.Context, order *domain.Order) (err error)
	GetByIdempotencyKey(ctx context.Context, idempotencyKey uuid.UUID) (order *domain.Order, err error)
	CreateItems(ctx context.Context, items []domain.OrderItem) (err error)
	GetByID(ctx context.Context, id uuid.UUID) (order *domain.Order, err error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.OrderStatus) (err error)
	GetListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Order, error)
	UpdateStatusIfCurrent(ctx context.Context, id uuid.UUID, newStatus, expectedStatus domain.OrderStatus) error
}

type SagaRepository interface {
	Create(ctx context.Context, saga *domain.SagaState) (err error)
	GetByOrderID(ctx context.Context, orderID uuid.UUID) (saga *domain.SagaState, err error)
	Update(ctx context.Context, saga *domain.SagaState) (err error)
}

type StateMachine interface {
	CanTransition(from, to string) bool
}

type InboxRepository interface {
	Create(ctx context.Context, event *domain.InboxEvent) (err error)
}

type OutboxRepository interface {
	Create(ctx context.Context, event *domain.OutboxEvent) (err error)
}

type OrderService struct {
	txManager  TxManager
	orderRepo  OrderRepository
	sagaRepo   SagaRepository
	outboxRepo OutboxRepository
	inboxRepo  InboxRepository
	productSvc ProductServiceClient

	orderStateMachine      StateMachine
	sagaStepStateMachine   StateMachine
	sagaStatusStateMachine StateMachine
}

func NewOrderService(
	txManager TxManager,
	orderRepo OrderRepository,
	sagaRepo SagaRepository,
	outboxRepo OutboxRepository,
	inboxRepo InboxRepository,
	productSvc ProductServiceClient,
	orderStateMachine StateMachine,
	sagaStepStateMachine StateMachine,
	sagaStatusStateMachine StateMachine,
) *OrderService {
	return &OrderService{
		txManager:              txManager,
		orderRepo:              orderRepo,
		sagaRepo:               sagaRepo,
		outboxRepo:             outboxRepo,
		inboxRepo:              inboxRepo,
		productSvc:             productSvc,
		orderStateMachine:      orderStateMachine,
		sagaStepStateMachine:   sagaStepStateMachine,
		sagaStatusStateMachine: sagaStatusStateMachine,
	}
}

func (s *OrderService) createOutboxEvent(
	ctx context.Context,
	correlationID uuid.UUID,
	aggregateID uuid.UUID,
	eventType string,
	payload map[string]any,
) error {
	event := &domain.OutboxEvent{
		ID:            uuid.New(),
		CorrelationID: correlationID,
		AggregateType: "order",
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       payload,
		CreatedAt:     time.Now(),
	}

	if err := s.outboxRepo.Create(ctx, event); err != nil {
		return apperror.NewInternal("outboxRepo.Create", err, "Что-то пошло не так", nil)
	}

	return nil
}
