package reservationService

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/domain"
	"github.com/google/uuid"
)

//go:generate mockgen -source=service.go -destination=mocks/mock_service.go -package=mocks -typed

type StockRepository interface {
	DecreaseQuantity(ctx context.Context, productID string, quantity int) error
	IncreaseQuantity(ctx context.Context, productID string, quantity int) error
}

type ReservationRepository interface {
	CreateBatch(ctx context.Context, reservations []*domain.Reservation) error
	GetActiveByOrderID(ctx context.Context, orderID uuid.UUID) ([]*domain.Reservation, error)
	ReleaseByOrderID(ctx context.Context, orderID uuid.UUID) (int, error)
	ExistsByOrderID(ctx context.Context, orderID uuid.UUID) (bool, error)
}

type OutboxRepository interface {
	Create(ctx context.Context, event *domain.OutboxEvent) error
}

type AdvisoryLocker interface {
	LockByUUID(ctx context.Context, id uuid.UUID) error
}

type TxManager interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

type ReservationService struct {
	stockRepo       StockRepository
	reservationRepo ReservationRepository
	outboxRepo      OutboxRepository
	locker          AdvisoryLocker
	txManager       TxManager
}

func NewReservationService(
	stockRepo StockRepository,
	reservationRepo ReservationRepository,
	outboxRepo OutboxRepository,
	locker AdvisoryLocker,
	txManager TxManager,
) *ReservationService {
	return &ReservationService{
		stockRepo:       stockRepo,
		reservationRepo: reservationRepo,
		outboxRepo:      outboxRepo,
		locker:          locker,
		txManager:       txManager,
	}
}

func (s *ReservationService) createOutboxEvent(
	ctx context.Context,
	correlationID uuid.UUID,
	aggregateID uuid.UUID,
	eventType string,
	payload map[string]any,
) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return apperror.NewInternal("json.Marshal outbox payload", err, "Что-то пошло не так", nil)
	}

	event := &domain.OutboxEvent{
		ID:            uuid.New(),
		CorrelationID: correlationID,
		AggregateType: "reservation",
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       payloadJSON,
		CreatedAt:     time.Now(),
	}

	if err := s.outboxRepo.Create(ctx, event); err != nil {
		if errors.Is(err, domain.AlreadyExistsError) {
			return err
		}
		return apperror.NewInternal("outboxRepo.Create", err, "Что-то пошло не так", nil)
	}

	return nil
}
