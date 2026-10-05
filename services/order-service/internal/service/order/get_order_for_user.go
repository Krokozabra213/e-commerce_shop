package orderService

import (
	"context"
	"errors"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/domain"
	"github.com/google/uuid"
)

func (s *OrderService) GetOrderForUser(ctx context.Context, orderID, currentUserID uuid.UUID, isAdmin bool) (*domain.Order, error) {
	order, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, apperror.NewBusiness(apperror.CodeNotFound, "order not found")
		}
		return nil, apperror.NewInternal("orderRepo.GetByID", err, "Что-то пошло не так", nil)
	}

	if !isAdmin && order.UserID != currentUserID {
		return nil, apperror.NewBusiness(apperror.CodeForbidden, "Нет доступа к заказу")
	}

	return order, nil
}
