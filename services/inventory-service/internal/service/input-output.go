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

//type CreateStockInput struct {
//	ProductID       string
//	InitialQuantity int
//}
//
//type AddStockInput struct {
//	ProductID string
//	Quantity  int
//}
