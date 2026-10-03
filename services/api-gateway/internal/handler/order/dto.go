package order

import (
	httpclient "github.com/Krokozabra213/e-commerce_shop/infra/clients/http"
	"github.com/google/uuid"
)

type createOrderRequest struct {
	Items []httpclient.OrderItem `json:"items" validate:"required,min=1,dive"`
}

type paymentSuccessRequest struct {
	EventID       uuid.UUID `json:"event_id" validate:"required"`
	CorrelationID uuid.UUID `json:"correlation_id" validate:"required"`
}

type paymentFailedRequest struct {
	EventID       uuid.UUID `json:"event_id" validate:"required"`
	CorrelationID uuid.UUID `json:"correlation_id" validate:"required"`
	Reason        string    `json:"reason" validate:"required"`
}
