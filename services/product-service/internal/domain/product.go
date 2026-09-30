package domain

import "time"

type Cursor struct {
	CreatedAt *time.Time `json:"createdAt,omitempty"`
	Price     *int64     `json:"price,omitempty"`
	ID        string     `json:"id"`
}

type SortType string

const (
	SortLatest    SortType = "latest"
	SortPriceAsc  SortType = "price_asc"
	SortPriceDesc SortType = "price_desc"
)

type ListProductsFilter struct {
	CategorySlug *string
	PriceMin     *int64
	PriceMax     *int64
	Search       *string
	Sort         SortType
	Cursor       *Cursor
	Limit        int
}

type Product struct {
	ID           string
	Name         string
	Description  *string
	Price        int64
	CategorySlug string
	Published    bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time
}

type CreateProductInput struct {
	Name         string
	Description  *string
	Price        int64
	CategorySlug string
	Published    bool
}

type UpdateProductInput struct {
	Name         *string
	Description  *string
	Price        *int64
	CategorySlug *string
	Published    *bool
}

type ProductPrice struct {
	ProductID string
	Price     int64
}
