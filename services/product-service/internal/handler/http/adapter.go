package httphandler

import (
	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/gofiber/fiber/v3"
)

func (h *Handler) ListProductsAdapter(c fiber.Ctx) error {
	params, err := ParseListProductsParams(c)
	if err != nil {
		return apperror.NewBusiness(apperror.CodeBadRequest, "invalid query parameters")
	}
	return h.ListProducts(c, *params)
}

func (h *Handler) GetProductAdapter(c fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return apperror.NewBusiness(apperror.CodeBadRequest, "product id is required")
	}
	return h.GetProduct(c, id)
}

func (h *Handler) UpdateProductAdapter(c fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return apperror.NewBusiness(apperror.CodeBadRequest, "product id is required")
	}
	return h.UpdateProduct(c, id)
}

func (h *Handler) DeleteProductAdapter(c fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return apperror.NewBusiness(apperror.CodeBadRequest, "product id is required")
	}
	return h.DeleteProduct(c, id)
}

func (h *Handler) TogglePublishAdapter(c fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return apperror.NewBusiness(apperror.CodeBadRequest, "product id is required")
	}
	return h.TogglePublish(c, id)
}

func (h *Handler) GetCategoryAdapter(c fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return apperror.NewBusiness(apperror.CodeBadRequest, "category slug is required")
	}
	return h.GetCategory(c, slug)
}

func (h *Handler) UpdateCategoryAdapter(c fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return apperror.NewBusiness(apperror.CodeBadRequest, "category slug is required")
	}
	return h.UpdateCategory(c, slug)
}

func (h *Handler) DeleteCategoryAdapter(c fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return apperror.NewBusiness(apperror.CodeBadRequest, "category slug is required")
	}
	return h.DeleteCategory(c, slug)
}
