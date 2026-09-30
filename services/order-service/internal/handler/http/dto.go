package httphandler

import (
	"time"

	"github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/domain"
	svcDTO "github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/service/dto"
	"github.com/google/uuid"
)

type paymentSucceededRequest struct {
	EventID       uuid.UUID `json:"event_id" validate:"required"`
	CorrelationID uuid.UUID `json:"correlation_id" validate:"required"`
}

type paymentFailedRequest struct {
	EventID       uuid.UUID `json:"event_id" validate:"required"`
	CorrelationID uuid.UUID `json:"correlation_id" validate:"required"`
	Reason        string    `json:"reason" validate:"required"`
}

type createOrderRequest struct {
	Items []struct {
		ProductID string `json:"product_id" validate:"required"`
		Quantity  int    `json:"quantity" validate:"required,min=1"`
	} `json:"items" validate:"required,min=1,dive"`
}

type createOrderResponse struct {
	OrderID    uuid.UUID `json:"order_id"`
	Status     string    `json:"status"`
	TotalPrice int64     `json:"total_price"`
}

func toCreateOrderResponse(out *svcDTO.CreateOrderOutput) createOrderResponse {
	return createOrderResponse{
		OrderID:    out.OrderID,
		Status:     out.Status,
		TotalPrice: out.TotalPrice,
	}
}

type orderResponse struct {
	OrderID    uuid.UUID          `json:"order_id"`
	UserID     uuid.UUID          `json:"user_id"`
	Status     domain.OrderStatus `json:"status"`
	TotalPrice int64              `json:"total_price"`
	CreatedAt  time.Time          `json:"created_at"`
	UpdatedAt  time.Time          `json:"updated_at"`
}

func toOrderResponse(o *domain.Order) orderResponse {
	return orderResponse{
		OrderID:    o.ID,
		UserID:     o.UserID,
		Status:     o.Status,
		TotalPrice: o.TotalPrice,
		CreatedAt:  o.CreatedAt,
		UpdatedAt:  o.UpdatedAt,
	}
}

func toOrderResponses(orders []domain.Order) []orderResponse {
	resp := make([]orderResponse, len(orders))
	for i, o := range orders {
		resp[i] = toOrderResponse(&o)
	}
	return resp
}
