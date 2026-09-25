package reservationService

import (
	"context"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/service"
)

func (s *ReservationService) Release(ctx context.Context, input service.ReleaseInput) error {
	err := s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := s.locker.LockByUUID(ctx, input.OrderID); err != nil {
			return apperror.NewInternal("locker.LockByUUID", err, "Что-то пошло не так", nil)
		}

		activeReservations, err := s.reservationRepo.GetActiveByOrderID(ctx, input.OrderID)
		if err != nil {
			return apperror.NewInternal("reservationRepo.GetActiveByOrderID", err, "Что-то пошло не так", nil)
		}

		if len(activeReservations) == 0 {
			return nil
		}

		for _, res := range activeReservations {
			if err := s.stockRepo.IncreaseQuantity(ctx, res.ProductID, res.Quantity); err != nil {
				return apperror.NewInternal("stockRepo.IncreaseQuantity", err, "Что-то пошло не так", nil)
			}
		}

		if _, err := s.reservationRepo.ReleaseByOrderID(ctx, input.OrderID); err != nil {
			return apperror.NewInternal("reservationRepo.ReleaseByOrderID", err, "Что-то пошло не так", nil)
		}

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}
