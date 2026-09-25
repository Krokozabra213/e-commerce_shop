package grpchandler

import (
	"context"

	inventoryv1 "github.com/Krokozabra213/e-commerce_shop/api/gen/go/proto/inventory/v1"
	"github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type StockService interface {
	GetByProductID(ctx context.Context, productID string) (*domain.Stock, error)
	GetQuantities(ctx context.Context, productIDs []string) (map[string]int, error)
}

type InventoryHandler struct {
	inventoryv1.UnimplementedInventoryServiceAPIServer
	stockService StockService
}

func NewInventoryHandler(stockService StockService) *InventoryHandler {
	return &InventoryHandler{
		stockService: stockService,
	}
}

func (h *InventoryHandler) GetStock(
	ctx context.Context,
	req *inventoryv1.GetStockRequest,
) (*inventoryv1.GetStockResponse, error) {
	if len(req.GetProductIds()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "product_ids не может быть пустым")
	}

	quantities, err := h.stockService.GetQuantities(ctx, req.GetProductIds())
	if err != nil {
		return nil, err
	}

	result := make(map[string]int32, len(quantities))
	for productID, quantity := range quantities {
		result[productID] = int32(quantity)
	}

	return &inventoryv1.GetStockResponse{
		Quantities: result,
	}, nil
}

func (h *InventoryHandler) GetStockByProductID(
	ctx context.Context,
	req *inventoryv1.GetStockByProductIDRequest,
) (*inventoryv1.GetStockByProductIDResponse, error) {
	if req.GetProductId() == "" {
		return nil, status.Error(codes.InvalidArgument, "product_id не может быть пустым")
	}

	stock, err := h.stockService.GetByProductID(ctx, req.GetProductId())
	if err != nil {
		return nil, err
	}

	return &inventoryv1.GetStockByProductIDResponse{
		Quantity: int32(stock.AvailableQuantity),
		InStock:  stock.AvailableQuantity > 0,
	}, nil
}
