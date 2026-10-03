package product

import (
	"context"
	"errors"
	"log/slog"
	"strconv"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	grpcclient "github.com/Krokozabra213/e-commerce_shop/infra/clients/grpc"
	httpclient "github.com/Krokozabra213/e-commerce_shop/infra/clients/http"
	"github.com/Krokozabra213/e-commerce_shop/services/api-gateway/internal/middleware"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"golang.org/x/sync/errgroup"
)

type ProdClient interface {
	ListProducts(ctx context.Context, params *httpclient.ListProductsParams) (*httpclient.ProductListResponse, error)
	GetProduct(ctx context.Context, id string) (*httpclient.Product, error)
	CreateProduct(ctx context.Context, auth *httpclient.AuthContext, input *httpclient.CreateProductRequest) (*httpclient.Product, error)
	UpdateProduct(ctx context.Context, auth *httpclient.AuthContext, id string, input *httpclient.UpdateProductRequest) (*httpclient.Product, error)
	DeleteProduct(ctx context.Context, auth *httpclient.AuthContext, id string) error
	TogglePublish(ctx context.Context, auth *httpclient.AuthContext, id string) (*httpclient.Product, error)

	ListCategories(ctx context.Context) (*httpclient.CategoryListResponse, error)
	GetCategory(ctx context.Context, slug string) (*httpclient.Category, error)
	CreateCategory(ctx context.Context, auth *httpclient.AuthContext, input *httpclient.CreateCategoryRequest) (*httpclient.Category, error)
	UpdateCategory(ctx context.Context, auth *httpclient.AuthContext, slug string, input *httpclient.UpdateCategoryRequest) (*httpclient.Category, error)
	DeleteCategory(ctx context.Context, auth *httpclient.AuthContext, slug string) error
}

type InventoryGRPCClient interface {
	GetStock(ctx context.Context, productIDs []string) (map[string]int32, error)
	GetStockByProductID(ctx context.Context, productID string) (*grpcclient.StockInfo, error)
}

type Handler struct {
	client    ProdClient
	inventory InventoryGRPCClient
	log       *slog.Logger
}

func NewHandler(client ProdClient, inventory InventoryGRPCClient, log *slog.Logger) *Handler {
	return &Handler{
		client:    client,
		inventory: inventory,
		log:       log,
	}
}

func (h *Handler) ListProducts(c fiber.Ctx) error {
	params := parseListProductsParams(c)

	resp, err := h.client.ListProducts(c.Context(), params)
	if err != nil {
		return err
	}

	stockMap := h.fetchStockMap(c.Context(), resp.Items)

	items := make([]ProductWithStock, len(resp.Items))
	for i, p := range resp.Items {
		items[i] = toProductWithStock(&p, stockMap[p.ID])
	}

	return c.Status(fiber.StatusOK).JSON(ProductListResponse{
		Items:      items,
		NextCursor: resp.NextCursor,
		HasMore:    resp.HasMore,
	})
}

func (h *Handler) GetProduct(c fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return apperror.NewBusiness(apperror.CodeBadRequest, "product id is required")
	}

	ctx := c.Context()

	var (
		product  *httpclient.Product
		quantity int32
	)

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var err error
		product, err = h.client.GetProduct(gCtx, id)
		return err
	})

	g.Go(func() error {
		stock, err := h.inventory.GetStockByProductID(gCtx, id)
		if err != nil {
			h.log.Warn("failed to fetch stock",
				slog.String("product_id", id),
				slog.String("error", err.Error()),
			)
			return nil
		}
		quantity = stock.Quantity
		return nil
	})

	if err := g.Wait(); err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(toProductWithStock(product, quantity))
}

func (h *Handler) CreateProduct(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "unauthorized")
	}

	roles, ok := middleware.UserRolesFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "missing user roles")
	}

	req := new(httpclient.CreateProductRequest)

	if err := c.Bind().Body(req); err != nil {
		var valErrs validator.ValidationErrors
		if errors.As(err, &valErrs) {
			return apperror.NewBusiness(apperror.CodeValidation, "Проверьте корректность данных")
		}
		return apperror.NewBusiness(apperror.CodeBadRequest, "Неверный формат запроса")
	}

	resp, err := h.client.CreateProduct(c.Context(), &httpclient.AuthContext{
		UserID: userID,
		Roles:  roles,
	}, req)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(resp)
}

func (h *Handler) UpdateProduct(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "unauthorized")
	}

	roles, ok := middleware.UserRolesFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "missing user roles")
	}

	id := c.Params("id")
	if id == "" {
		return apperror.NewBusiness(apperror.CodeBadRequest, "product id is required")
	}

	req := new(httpclient.UpdateProductRequest)

	if err := c.Bind().Body(req); err != nil {
		var valErrs validator.ValidationErrors
		if errors.As(err, &valErrs) {
			return apperror.NewBusiness(apperror.CodeValidation, "Проверьте корректность данных")
		}
		return apperror.NewBusiness(apperror.CodeBadRequest, "Неверный формат запроса")
	}

	resp, err := h.client.UpdateProduct(c.Context(), &httpclient.AuthContext{
		UserID: userID,
		Roles:  roles,
	}, id, req)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (h *Handler) DeleteProduct(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "unauthorized")
	}

	roles, ok := middleware.UserRolesFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "missing user roles")
	}

	id := c.Params("id")
	if id == "" {
		return apperror.NewBusiness(apperror.CodeBadRequest, "product id is required")
	}

	if err := h.client.DeleteProduct(c.Context(), &httpclient.AuthContext{
		UserID: userID,
		Roles:  roles,
	}, id); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) TogglePublish(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "unauthorized")
	}

	roles, ok := middleware.UserRolesFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "missing user roles")
	}

	id := c.Params("id")
	if id == "" {
		return apperror.NewBusiness(apperror.CodeBadRequest, "product id is required")
	}

	resp, err := h.client.TogglePublish(c.Context(), &httpclient.AuthContext{
		UserID: userID,
		Roles:  roles,
	}, id)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (h *Handler) ListCategories(c fiber.Ctx) error {
	resp, err := h.client.ListCategories(c.Context())
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (h *Handler) GetCategory(c fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return apperror.NewBusiness(apperror.CodeBadRequest, "category slug is required")
	}

	resp, err := h.client.GetCategory(c.Context(), slug)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (h *Handler) CreateCategory(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "unauthorized")
	}

	roles, ok := middleware.UserRolesFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "missing user roles")
	}

	req := new(httpclient.CreateCategoryRequest)

	if err := c.Bind().Body(req); err != nil {
		var valErrs validator.ValidationErrors
		if errors.As(err, &valErrs) {
			return apperror.NewBusiness(apperror.CodeValidation, "Проверьте корректность данных")
		}
		return apperror.NewBusiness(apperror.CodeBadRequest, "Неверный формат запроса")
	}

	resp, err := h.client.CreateCategory(c.Context(), &httpclient.AuthContext{
		UserID: userID,
		Roles:  roles,
	}, req)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(resp)
}

func (h *Handler) UpdateCategory(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "unauthorized")
	}

	roles, ok := middleware.UserRolesFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "missing user roles")
	}

	slug := c.Params("slug")
	if slug == "" {
		return apperror.NewBusiness(apperror.CodeBadRequest, "category slug is required")
	}

	req := new(httpclient.UpdateCategoryRequest)

	if err := c.Bind().Body(req); err != nil {
		var valErrs validator.ValidationErrors
		if errors.As(err, &valErrs) {
			return apperror.NewBusiness(apperror.CodeValidation, "Проверьте корректность данных")
		}
		return apperror.NewBusiness(apperror.CodeBadRequest, "Неверный формат запроса")
	}

	resp, err := h.client.UpdateCategory(c.Context(), &httpclient.AuthContext{
		UserID: userID,
		Roles:  roles,
	}, slug, req)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (h *Handler) DeleteCategory(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "unauthorized")
	}

	roles, ok := middleware.UserRolesFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "missing user roles")
	}

	slug := c.Params("slug")
	if slug == "" {
		return apperror.NewBusiness(apperror.CodeBadRequest, "category slug is required")
	}

	if err := h.client.DeleteCategory(c.Context(), &httpclient.AuthContext{
		UserID: userID,
		Roles:  roles,
	}, slug); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func parseListProductsParams(c fiber.Ctx) *httpclient.ListProductsParams {
	params := httpclient.ListProductsParams{
		Category: c.Query("category"),
		Search:   c.Query("search"),
		Sort:     c.Query("sort", "latest"),
		Cursor:   c.Query("cursor"),
	}

	if v := c.Query("price_min"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			params.PriceMin = &n
		}
	}

	if v := c.Query("price_max"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			params.PriceMax = &n
		}
	}

	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			params.Limit = n
		}
	}

	return &params
}

func (h *Handler) fetchStockMap(ctx context.Context, products []httpclient.Product) map[string]int32 {
	if len(products) == 0 {
		return map[string]int32{}
	}

	ids := make([]string, len(products))
	for i, p := range products {
		ids[i] = p.ID
	}

	stockMap, err := h.inventory.GetStock(ctx, ids)
	if err != nil {
		h.log.Warn("failed to fetch stock batch, returning empty map",
			slog.Int("count", len(ids)),
			slog.String("error", err.Error()),
		)
		return map[string]int32{}
	}

	return stockMap
}
