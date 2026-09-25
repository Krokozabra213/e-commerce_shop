package httphandler

import (
	"context"
	"log/slog"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/domain"
	"github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/service"
	"github.com/gofiber/fiber/v3"
)

type StockService interface {
	Create(ctx context.Context, input service.ProductItem) error
	GetByProductID(ctx context.Context, productID string) (*domain.Stock, error)
	AddStock(ctx context.Context, input service.ProductItem) error
}

type StockHandler struct {
	stockService StockService
	logger       *slog.Logger
}

func NewStockHandler(
	stockService StockService,
	logger *slog.Logger,
) *StockHandler {
	return &StockHandler{
		stockService: stockService,
		logger:       logger,
	}
}

func (h *StockHandler) CreateStock(c fiber.Ctx) error {
	var req CreateStockRequest

	if err := c.Bind().JSON(&req); err != nil {
		return apperror.NewBusiness(apperror.CodeBadRequest, "invalid request body")
	}

	input := service.ProductItem{
		ProductID: req.ProductID,
		Quantity:  req.InitialQuantity,
	}

	if err := h.stockService.Create(c.Context(), input); err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(MessageResponse{
		Message: "Запись о товаре успешно создана",
	})
}

func (h *StockHandler) GetStock(c fiber.Ctx) error {
	productID := c.Params("productId")

	if productID == "" {
		return apperror.NewBusiness(
			apperror.CodeBadRequest,
			"product_id обязателен",
		)
	}

	stock, err := h.stockService.GetByProductID(c.Context(), productID)
	if err != nil {
		return err
	}

	return c.JSON(StockResponse{
		ProductID:         stock.ProductID,
		AvailableQuantity: stock.AvailableQuantity,
		InStock:           stock.AvailableQuantity > 0,
	})
}

func (h *StockHandler) AddStock(c fiber.Ctx) error {
	productID := c.Params("productId")

	if productID == "" {
		return apperror.NewBusiness(
			apperror.CodeBadRequest,
			"product_id обязателен",
		)
	}

	var req AddStockRequest

	if err := c.Bind().JSON(&req); err != nil {
		return apperror.NewBusiness(apperror.CodeBadRequest, "invalid request body")
	}

	input := service.ProductItem{
		ProductID: productID,
		Quantity:  req.Quantity,
	}

	if err := h.stockService.AddStock(c.Context(), input); err != nil {
		return err
	}

	return c.JSON(MessageResponse{
		Message: "Количество товара успешно увеличено",
	})
}
