package inventory

type createStockRequest struct {
	ProductID       string `json:"product_id" validate:"required"`
	InitialQuantity int    `json:"initial_quantity" validate:"min=0"`
}

type addStockRequest struct {
	Quantity int `json:"quantity" validate:"required,min=1"`
}
