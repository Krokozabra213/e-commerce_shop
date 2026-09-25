package reservationService

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	infrakafka "github.com/Krokozabra213/e-commerce_shop/infra/kafka"
	"github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/domain"
	"github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/service"
	"github.com/google/uuid"
)

func (s *ReservationService) Reserve(ctx context.Context, input service.ReserveInput) error {

	sort.Slice(input.Items, func(i, j int) bool {
		return input.Items[i].ProductID < input.Items[j].ProductID
	})

	reserveErr := s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := s.locker.LockByUUID(ctx, input.OrderID); err != nil {
			return apperror.NewInternal("locker.LockOrderID", err, "Что-то пошло не так", nil)
		}

		alreadyProcessed, err := s.reservationRepo.ExistsByOrderID(ctx, input.OrderID)
		if err != nil {
			return apperror.NewInternal("reservationRepo.ExistsByOrderID", err, "Что-то пошло не так", nil)
		}
		if alreadyProcessed {
			return nil
		}

		now := time.Now()
		reservations := make([]*domain.Reservation, 0, len(input.Items))

		for _, item := range input.Items {
			err := s.stockRepo.DecreaseQuantity(ctx, item.ProductID, item.Quantity)
			if err != nil {
				if errors.Is(err, domain.InsufficientStockError) {
					return apperror.NewAppErr(apperror.CodeConflict, "stockRepo.DecreaseQuantity", "не хватает товара", err, apperror.LevelDebug, nil)
				}
				if errors.Is(err, domain.NotFoundError) {
					return apperror.NewAppErr(apperror.CodeBadRequest, "stockRepo.DecreaseQuantity", "товар не найден", err, apperror.LevelDebug, nil)
				}
				return apperror.NewInternal("stockRepo.DecreaseQuantity", err, "Что-то пошло не так", nil)
			}

			reservations = append(reservations, &domain.Reservation{
				ID:        uuid.New(),
				OrderID:   input.OrderID,
				ProductID: item.ProductID,
				Quantity:  item.Quantity,
				Status:    domain.ReservationStatusActive,
				CreatedAt: now,
			})
		}

		if err := s.reservationRepo.CreateBatch(ctx, reservations); err != nil {
			return apperror.NewInternal("reservationRepo.CreateBatch", err, "Что-то пошло не так", nil)
		}
		if err := s.publishReserveSuccess(ctx, input); err != nil {
			return err
		}

		return nil
	})

	if reserveErr == nil {
		return nil
	}

	failedReason := domain.ReservationFailedReasonInternal
	if errors.Is(reserveErr, domain.InsufficientStockError) {
		failedReason = domain.ReservationFailedReasonInsufficientStock
	} else if errors.Is(reserveErr, domain.NotFoundError) {
		failedReason = domain.ReservationFailedReasonNotFound
	}

	if err := s.publishReserveFailed(ctx, input, failedReason.String()); err != nil {
		if errors.Is(err, domain.AlreadyExistsError) {
			return nil
		}
		return fmt.Errorf("reserve failed and outbox write failed: reserve=%w, outbox=%v", reserveErr, err)
	}

	return nil
}

func (s *ReservationService) publishReserveSuccess(ctx context.Context, input service.ReserveInput) error {
	items := make([]map[string]any, 0, len(input.Items))
	for _, item := range input.Items {
		items = append(items, map[string]any{
			"product_id": item.ProductID,
			"quantity":   item.Quantity,
		})
	}

	payload := map[string]any{
		"correlation_id": input.CorrelationID,
		"order_id":       input.OrderID,
		"items":          items,
	}

	return s.createOutboxEvent(ctx, input.CorrelationID, input.OrderID, infrakafka.TopicInventoryReserved, payload)
}

func (s *ReservationService) publishReserveFailed(ctx context.Context, input service.ReserveInput, reason string) error {
	payload := map[string]any{
		"correlation_id": input.CorrelationID,
		"order_id":       input.OrderID,
		"reason":         reason,
	}

	return s.createOutboxEvent(ctx, input.CorrelationID, input.OrderID, infrakafka.TopicInventoryReservFailed, payload)
}
