package httphandler

import (
	"github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/handler/http/generated"
)

type Handler struct {
	*ProductHandler
	*CategoryHandler
}

func NewHandler(
	productService ProductService,
	categoryService CategoryService,
) *Handler {
	return &Handler{
		ProductHandler:  NewProductHandler(productService),
		CategoryHandler: NewCategoryHandler(categoryService),
	}
}

var _ generated.ServerInterface = (*Handler)(nil)
