package service

import "github.com/google/uuid"

type ReserveInput struct {
	CorrelationID uuid.UUID
	OrderID       uuid.UUID
	Items         []ProductItem
}

type ProductItem struct {
	ProductID string
	Quantity  int
}

type ReleaseInput struct {
	CorrelationID uuid.UUID
	OrderID       uuid.UUID
}
