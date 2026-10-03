package inventory

import (
	"context"
	"errors"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	httpclient "github.com/Krokozabra213/e-commerce_shop/infra/clients/http"
	"github.com/Krokozabra213/e-commerce_shop/services/api-gateway/internal/middleware"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
)

type InventoryClient interface {
	GetStock(ctx context.Context, productID string) (*httpclient.StockResponse, error)
	CreateStock(ctx context.Context, auth *httpclient.AuthContext, productID string, initialQuantity int) (*httpclient.MessageResponse, error)
	AddStock(ctx context.Context, auth *httpclient.AuthContext, productID string, quantity int) (*httpclient.MessageResponse, error)
}

type Handler struct {
	Client InventoryClient
}

func NewHandler(client InventoryClient) *Handler {
	return &Handler{Client: client}
}

func (h *Handler) GetStock(c fiber.Ctx) error {
	productID := c.Params("productId")
	if productID == "" {
		return apperror.NewBusiness(apperror.CodeBadRequest, "product id is required")
	}

	resp, err := h.Client.GetStock(c.Context(), productID)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (h *Handler) CreateStock(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "unauthorized")
	}

	roles, ok := middleware.UserRolesFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "missing user roles")
	}

	req := new(createStockRequest)

	if err := c.Bind().Body(req); err != nil {
		var valErrs validator.ValidationErrors
		if errors.As(err, &valErrs) {
			return apperror.NewBusiness(apperror.CodeValidation, "Проверьте корректность данных")
		}
		return apperror.NewBusiness(apperror.CodeBadRequest, "Неверный формат запроса")
	}

	resp, err := h.Client.CreateStock(c.Context(), &httpclient.AuthContext{
		UserID: userID,
		Roles:  roles,
	}, req.ProductID, req.InitialQuantity)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(resp)
}

func (h *Handler) AddStock(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}

	roles, ok := middleware.UserRolesFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "missing user roles")
	}

	productID := c.Params("productId")
	if productID == "" {
		return apperror.NewBusiness(apperror.CodeBadRequest, "product id is required")
	}

	req := new(addStockRequest)

	if err := c.Bind().Body(req); err != nil {
		var valErrs validator.ValidationErrors
		if errors.As(err, &valErrs) {
			return apperror.NewBusiness(apperror.CodeValidation, "Проверьте корректность данных")
		}
		return apperror.NewBusiness(apperror.CodeBadRequest, "Неверный формат запроса")
	}

	resp, err := h.Client.AddStock(c.Context(), &httpclient.AuthContext{
		UserID: userID,
		Roles:  roles,
	}, productID, req.Quantity)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}
