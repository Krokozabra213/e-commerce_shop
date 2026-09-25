package httphandler

type CreateStockRequest struct {
	ProductID       string `json:"product_id" validate:"required"`
	InitialQuantity int    `json:"initial_quantity" validate:"min=0"`
}

type AddStockRequest struct {
	Quantity int `json:"quantity" validate:"required,min=1"`
}

type StockResponse struct {
	ProductID         string `json:"product_id"`
	AvailableQuantity int    `json:"available_quantity"`
	InStock           bool   `json:"in_stock"`
}

type MessageResponse struct {
	Message string `json:"message"`
}
