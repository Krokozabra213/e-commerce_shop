package product

import (
	"time"

	httpclient "github.com/Krokozabra213/e-commerce_shop/infra/clients/http"
)

type ProductWithStock struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  *string   `json:"description"`
	Price        int64     `json:"price"`
	CategorySlug string    `json:"category_slug"`
	Published    bool      `json:"published"`
	Quantity     int32     `json:"quantity"`
	InStock      bool      `json:"in_stock"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ProductListResponse struct {
	Items      []ProductWithStock `json:"items"`
	NextCursor *string            `json:"next_cursor"`
	HasMore    bool               `json:"has_more"`
}

func toProductWithStock(p *httpclient.Product, quantity int32) ProductWithStock {
	return ProductWithStock{
		ID:           p.ID,
		Name:         p.Name,
		Description:  p.Description,
		Price:        p.Price,
		CategorySlug: p.CategorySlug,
		Published:    p.Published,
		Quantity:     quantity,
		InStock:      quantity > 0,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}
}
