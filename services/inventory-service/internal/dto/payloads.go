package dto

type InventoryReservedPayload struct {
	OrderID string `json:"order_id"`
}

type InventoryReservationFailedPayload struct {
	OrderID string `json:"order_id"`
	Reason  string `json:"reason"`
}
