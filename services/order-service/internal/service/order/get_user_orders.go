package orderService

import (
	"context"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/domain"
	"github.com/google/uuid"
)

func (s *OrderService) GetUserOrders(ctx context.Context, currentUserID uuid.UUID) ([]domain.Order, error) {
	orders, err := s.orderRepo.GetListByUserID(ctx, currentUserID)
	if err != nil {
		return nil, apperror.NewInternal("orderRepo.GetListByUserID", err, "Что-то пошло не так", nil)
	}

	return orders, nil
}
