package httphandler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/domain"
	"github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/handler/http/generated"
	"github.com/gofiber/fiber/v3"
)

type ProductService interface {
	Create(ctx context.Context, input *domain.CreateProductInput) (*domain.Product, error)
	GetByID(ctx context.Context, id string) (*domain.Product, error)
	List(ctx context.Context, filter *domain.ListProductsFilter) ([]domain.Product, *domain.Cursor, error)
	Update(ctx context.Context, id string, input *domain.UpdateProductInput) (*domain.Product, error)
	Delete(ctx context.Context, id string) error
	TogglePublish(ctx context.Context, id string) (*domain.Product, error)
}

type ProductHandler struct {
	productService ProductService
}

func NewProductHandler(productService ProductService) *ProductHandler {
	return &ProductHandler{
		productService: productService,
	}
}

func (h *ProductHandler) ListProducts(c fiber.Ctx, params generated.ListProductsParams) error {
	var cursor *domain.Cursor
	if params.Cursor != nil && *params.Cursor != "" {
		var err error
		cursor, err = decodeCursor(*params.Cursor)
		if err != nil {
			return apperror.NewBusiness(apperror.CodeValidation, "cursor is invalid")
		}
	}

	sort := mapSortToDomain(*params.Sort)

	filter := domain.ListProductsFilter{
		CategorySlug: params.Category,
		PriceMin:     params.PriceMin,
		PriceMax:     params.PriceMax,
		Search:       params.Search,
		Sort:         sort,
		Cursor:       cursor,
		Limit:        *params.Limit,
	}

	products, nextCursor, err := h.productService.List(c.Context(), &filter)
	if err != nil {
		return err
	}

	items := make([]generated.Product, len(products))
	for i, p := range products {
		items[i] = mapProductToAPI(p)
	}

	response := generated.ProductListResponse{
		Items: items,
	}

	if nextCursor != nil {
		encoded := encodeCursor(nextCursor)
		response.NextCursor = &encoded
		response.HasMore = true
	}

	return c.JSON(response)
}

func (h *ProductHandler) GetProduct(c fiber.Ctx, id string) error {
	ctx := c.Context()

	product, err := h.productService.GetByID(ctx, id)
	if err != nil {
		return err
	}

	return c.JSON(mapProductToAPI(*product))
}

func (h *ProductHandler) CreateProduct(c fiber.Ctx) error {
	ctx := c.Context()

	var req generated.CreateProductRequest
	if err := c.Bind().Body(&req); err != nil {
		return apperror.NewBusiness(apperror.CodeBadRequest, "invalid request body")
	}

	if req.Price < 0 {
		return apperror.NewBusiness(apperror.CodeBadRequest, "price must be non-negative")
	}

	if req.CategorySlug == "" {
		return apperror.NewBusiness(apperror.CodeBadRequest, "category slug is required")
	}

	input := domain.CreateProductInput{
		Name:         req.Name,
		Description:  req.Description,
		Price:        req.Price,
		CategorySlug: req.CategorySlug,
		Published:    req.Published != nil && *req.Published,
	}

	product, err := h.productService.Create(ctx, &input)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(mapProductToAPI(*product))
}

func (h *ProductHandler) UpdateProduct(c fiber.Ctx, id string) error {
	ctx := c.Context()

	var req generated.UpdateProductRequest
	if err := c.Bind().Body(&req); err != nil {
		return apperror.NewBusiness(apperror.CodeBadRequest, "invalid request body")
	}

	if req.Price != nil && *req.Price < 0 {
		return apperror.NewBusiness(apperror.CodeBadRequest, "price must be non-negative")
	}

	if req.CategorySlug != nil && *req.CategorySlug == "" {
		return apperror.NewBusiness(apperror.CodeBadRequest, "category slug must be non-empty")
	}

	input := domain.UpdateProductInput{
		Name:         req.Name,
		Description:  req.Description,
		Price:        req.Price,
		CategorySlug: req.CategorySlug,
		Published:    req.Published,
	}

	product, err := h.productService.Update(ctx, id, &input)
	if err != nil {
		return err
	}

	return c.JSON(mapProductToAPI(*product))
}

func (h *ProductHandler) DeleteProduct(c fiber.Ctx, id string) error {
	ctx := c.Context()

	err := h.productService.Delete(ctx, id)
	if err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (h *ProductHandler) TogglePublish(c fiber.Ctx, id string) error {
	ctx := c.Context()

	product, err := h.productService.TogglePublish(ctx, id)
	if err != nil {
		return err
	}

	return c.JSON(mapProductToAPI(*product))
}

func encodeCursor(cursor *domain.Cursor) string {
	data, _ := json.Marshal(cursor)
	return base64.URLEncoding.EncodeToString(data)
}

func decodeCursor(encoded string) (*domain.Cursor, error) {
	data, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("invalid base64: %w", err)
	}

	var cursor domain.Cursor
	if err := json.Unmarshal(data, &cursor); err != nil {
		return nil, fmt.Errorf("invalid cursor format: %w", err)
	}

	return &cursor, nil
}

func mapProductToAPI(p domain.Product) generated.Product {
	return generated.Product{
		Id:           p.ID,
		Name:         p.Name,
		Description:  p.Description,
		Price:        p.Price,
		CategorySlug: p.CategorySlug,
		Published:    p.Published,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}
}

func mapSortToDomain(sort generated.ListProductsParamsSort) domain.SortType {
	switch sort {
	case generated.ListProductsParamsSortLatest:
		return domain.SortLatest
	case generated.ListProductsParamsSortPriceAsc:
		return domain.SortPriceAsc
	case generated.ListProductsParamsSortPriceDesc:
		return domain.SortPriceDesc
	default:
		return domain.SortLatest
	}
}
