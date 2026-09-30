package svcDTO

import (
	"encoding/json"
	"time"
)

type OrderCreatedPayload struct {
	OrderID   string             `json:"order_id"`
	Items     []OrderItemPayload `json:"items"`
	CreatedAt time.Time          `json:"created_at"`
}

type OrderItemPayload struct {
	ProductID string `json:"product_id"`
	Quantity  int32  `json:"quantity"`
}

type PaymentChargePayload struct {
	OrderID     string    `json:"order_id"`
	UserID      string    `json:"user_id"`
	Amount      int64     `json:"amount"`
	RequestedAt time.Time `json:"requested_at"`
}

type OrderCancellationPayload struct {
	OrderID     string    `json:"order_id"`
	Reason      string    `json:"reason"`
	CancelledAt time.Time `json:"cancelled_at"`
}

type PaymentFailedPayload struct {
	OrderID string `json:"order_id"`
	Reason  string `json:"reason"`
}

func ToMap(v any) (map[string]any, error) {
	bytes, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(bytes, &m); err != nil {
		return nil, err
	}
	return m, nil
}
