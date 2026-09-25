package httphandler

import (
	"log/slog"

	"github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/handler/http/generated"
)

type Handler struct {
	*ProductHandler
	*CategoryHandler
}

func NewHandler(
	productService ProductService,
	categoryService CategoryService,
	log *slog.Logger,
) *Handler {
	return &Handler{
		ProductHandler:  NewProductHandler(productService, log),
		CategoryHandler: NewCategoryHandler(categoryService, log),
	}
}

var _ generated.ServerInterface = (*Handler)(nil)
