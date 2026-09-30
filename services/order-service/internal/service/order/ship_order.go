package orderService

import (
	"context"
	"errors"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/domain"
	"github.com/google/uuid"
)

func (s *OrderService) ShipOrder(ctx context.Context, id uuid.UUID) error {
	err := s.orderRepo.UpdateStatusIfCurrent(ctx, id, domain.OrderStatusShipped, domain.OrderStatusPaid)
	if err != nil {
		if errors.Is(err, domain.ErrMismatchOrNotFound) {
			return apperror.NewBusiness(apperror.CodeConflict, "Переход не возможен")
		}
		return apperror.NewInternal("orderRepo.UpdateStatusIfCurrent", err, "Что-то пошло не так", nil)
	}
	return nil
}
