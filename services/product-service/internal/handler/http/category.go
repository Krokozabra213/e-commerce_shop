package httphandler

import (
	"context"
	"log/slog"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/domain"
	"github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/handler/http/generated"
	"github.com/gofiber/fiber/v3"
)

type CategoryService interface {
	Create(ctx context.Context, input domain.CreateCategoryInput) (*domain.Category, error)
	GetBySlug(ctx context.Context, slug string) (*domain.Category, error)
	List(ctx context.Context) ([]domain.Category, error)
	Update(ctx context.Context, slug string, input domain.UpdateCategoryInput) (*domain.Category, error)
	Delete(ctx context.Context, slug string) error
}

type CategoryHandler struct {
	categoryService CategoryService
	log             *slog.Logger
}

func NewCategoryHandler(categoryService CategoryService, log *slog.Logger) *CategoryHandler {
	return &CategoryHandler{
		categoryService: categoryService,
		log:             log,
	}
}

func (h *CategoryHandler) ListCategories(c fiber.Ctx) error {
	ctx := c.Context()

	categories, err := h.categoryService.List(ctx)
	if err != nil {
		return err
	}

	items := make([]generated.Category, len(categories))
	for i, cat := range categories {
		items[i] = mapCategoryToAPI(&cat)
	}

	return c.JSON(generated.CategoryListResponse{
		Items: items,
	})
}

func (h *CategoryHandler) GetCategory(c fiber.Ctx, slug string) error {
	ctx := c.Context()

	category, err := h.categoryService.GetBySlug(ctx, slug)
	if err != nil {
		return err
	}

	return c.JSON(mapCategoryToAPI(category))
}

func (h *CategoryHandler) CreateCategory(c fiber.Ctx) error {
	ctx := c.Context()

	var req generated.CreateCategoryRequest
	if err := c.Bind().Body(&req); err != nil {
		return apperror.NewBusiness(apperror.CodeBadRequest, "invalid request body")
	}

	input := domain.CreateCategoryInput{
		Name:        req.Name,
		Slug:        req.Slug,
		Description: stringPtrToValue(req.Description),
	}

	category, err := h.categoryService.Create(ctx, input)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(mapCategoryToAPI(category))
}

func (h *CategoryHandler) UpdateCategory(c fiber.Ctx, slug string) error {
	ctx := c.Context()

	var req generated.UpdateCategoryRequest
	if err := c.Bind().Body(&req); err != nil {
		return apperror.NewBusiness(apperror.CodeBadRequest, "invalid request body")
	}

	input := domain.UpdateCategoryInput{
		Name:        req.Name,
		Description: req.Description,
	}

	category, err := h.categoryService.Update(ctx, slug, input)
	if err != nil {
		return err
	}

	return c.JSON(mapCategoryToAPI(category))
}

func (h *CategoryHandler) DeleteCategory(c fiber.Ctx, slug string) error {
	ctx := c.Context()

	err := h.categoryService.Delete(ctx, slug)
	if err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func mapCategoryToAPI(c *domain.Category) generated.Category {
	return generated.Category{
		Id:          c.ID,
		Name:        c.Name,
		Slug:        c.Slug,
		Description: &c.Description,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}

func stringPtrToValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
