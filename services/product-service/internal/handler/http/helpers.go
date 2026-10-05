package httphandler

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/handler/http/generated"
	"github.com/gofiber/fiber/v3"
)

const (
	DefaultLimit = 20
	MinLimit     = 1
	MaxLimit     = 100
)

func ParseListProductsParams(c fiber.Ctx) (*generated.ListProductsParams, error) {
	var p generated.ListProductsParams

	if v := c.Query("category"); v != "" {
		p.Category = &v
	}
	if v := c.Query("search"); v != "" {
		p.Search = &v
	}
	if v := c.Query("cursor"); v != "" {
		p.Cursor = &v
	}

	if v := c.Query("sort"); v != "" {
		s := generated.ListProductsParamsSort(v)
		if !s.Valid() {
			return nil, fmt.Errorf("invalid sort: %q (must be one of: latest, price_asc, price_desc)", v)
		}
		p.Sort = &s
	} else {
		defaultSort := generated.ListProductsParamsSortLatest
		p.Sort = &defaultSort
	}

	if v := c.Query("limit"); v != "" {
		l, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid limit: %q (must be integer)", v)
		}
		if l < MinLimit || l > MaxLimit {
			return nil, fmt.Errorf("limit must be between %d and %d", MinLimit, MaxLimit)
		}
		p.Limit = &l
	} else {
		limit := DefaultLimit
		p.Limit = &limit
	}

	if v := c.Query("price_min"); v != "" {
		minPrice, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid price_min: %q (must be integer)", v)
		}
		if minPrice < 0 {
			return nil, errors.New("price_min must be non-negative")
		}
		p.PriceMin = &minPrice
	}

	if v := c.Query("price_max"); v != "" {
		maxPrice, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid price_max: %q (must be integer)", v)
		}
		if maxPrice < 0 {
			return nil, errors.New("price_max must be non-negative")
		}
		p.PriceMax = &maxPrice
	}

	if p.PriceMin != nil && p.PriceMax != nil && *p.PriceMin > *p.PriceMax {
		return nil, errors.New("price_min cannot be greater than price_max")
	}

	return &p, nil
}
