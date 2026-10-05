package grpchandler

import (
	"context"

	productv1 "github.com/Krokozabra213/e-commerce_shop/api/gen/go/proto/product/v1"
	"github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ProductService interface {
	GetPricesByIDs(ctx context.Context, ids []string) ([]domain.ProductPrice, error)
}

type ProductHandler struct {
	productv1.UnimplementedProductServiceAPIServer
	productService ProductService
}

func NewProductHandler(productService ProductService) *ProductHandler {
	return &ProductHandler{
		productService: productService,
	}
}

func (h *ProductHandler) GetPricesByProductIDs(
	ctx context.Context,
	req *productv1.GetPricesByProductIDsRequest,
) (*productv1.GetPricesByProductIDsResponse, error) {
	if len(req.GetProductIds()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "product_ids не может быть пустым")
	}

	for i, id := range req.GetProductIds() {
		if id == "" {
			return nil, status.Errorf(codes.InvalidArgument, "product_ids[%d] не может быть пустым", i)
		}
	}

	prices, err := h.productService.GetPricesByIDs(ctx, req.GetProductIds())
	if err != nil {
		return nil, err
	}

	pricesMap := make(map[string]int64, len(prices))
	for _, p := range prices {
		pricesMap[p.ProductID] = p.Price
	}

	return &productv1.GetPricesByProductIDsResponse{
		Prices: pricesMap,
	}, nil
}
