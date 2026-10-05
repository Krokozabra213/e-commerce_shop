package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	infracfg "github.com/Krokozabra213/e-commerce_shop/infra/config"
	"github.com/Krokozabra213/e-commerce_shop/infra/httpx"
	"github.com/google/uuid"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type Product struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  *string   `json:"description"`
	Price        int64     `json:"price"`
	CategorySlug string    `json:"categorySlug"`
	Published    bool      `json:"published"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type ProductListResponse struct {
	Items      []Product `json:"items"`
	NextCursor *string   `json:"nextCursor"`
	HasMore    bool      `json:"hasMore"`
}

type ListProductsParams struct {
	Category string
	PriceMin *int64
	PriceMax *int64
	Search   string
	Sort     string
	Cursor   string
	Limit    int
}

type CreateProductRequest struct {
	Name         string  `json:"name"`
	Description  *string `json:"description,omitempty"`
	Price        int64   `json:"price"`
	CategorySlug string  `json:"categorySlug"`
	Published    bool    `json:"published"`
}

type UpdateProductRequest struct {
	Name         *string `json:"name,omitempty"`
	Description  *string `json:"description,omitempty"`
	Price        *int64  `json:"price,omitempty"`
	CategorySlug *string `json:"categorySlug,omitempty"`
	Published    *bool   `json:"published,omitempty"`
}

type Category struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description *string   `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type CategoryListResponse struct {
	Items []Category `json:"items"`
}

type CreateCategoryRequest struct {
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Description *string `json:"description,omitempty"`
}

type UpdateCategoryRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

type AuthContext struct {
	UserID uuid.UUID
	Roles  []string
}

type ProductClient struct {
	httpClient *http.Client
	config     infracfg.HTTPClientConfig
}

func NewProductClient(cfg infracfg.HTTPClientConfig) *ProductClient {
	addr := strings.TrimRight(cfg.Addr, "/")

	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		addr = "http://" + addr
	}

	cfg.Addr = addr + "/"

	baseTransport := WithRequestID(http.DefaultTransport)
	instrumentedTransport := otelhttp.NewTransport(baseTransport)

	return &ProductClient{
		httpClient: &http.Client{
			Timeout:   cfg.Timeout,
			Transport: instrumentedTransport,
		},
		config: cfg,
	}
}

func (c *ProductClient) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.config.Addr+"healthz", nil)
	if err != nil {
		return fmt.Errorf("healthcheck: new request: %w", err)
	}

	return c.do("HealthCheck", req, nil, http.StatusOK)
}

func (c *ProductClient) ReadyCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.config.Addr+"readyz", nil)
	if err != nil {
		return fmt.Errorf("readycheck: new request: %w", err)
	}

	return c.do("ReadyCheck", req, nil, http.StatusOK)
}

func (c *ProductClient) do(op string, req *http.Request, dst any, expectedStatus int) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return apperror.NewInternal(op, err, "Не удалось связаться с product-service", nil)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != expectedStatus {
		return ParseDownstreamError("product-service", resp)
	}

	if dst == nil || expectedStatus == http.StatusNoContent {
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return apperror.NewInternal(op, err, "Не удалось декодировать ответ product-service", nil)
	}

	return nil
}

func (c *ProductClient) newRequest(
	ctx context.Context,
	op, method, path string,
	body []byte,
	auth *AuthContext,
) (*http.Request, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.config.Addr+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("%s: new request: %w", op, err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if auth != nil {
		req.Header.Set(httpx.HeaderUserID, auth.UserID.String())
		req.Header.Set(httpx.HeaderUserRoles, strings.Join(auth.Roles, ","))
	}

	return req, nil
}

func (c *ProductClient) postJSON(
	ctx context.Context,
	op, path string,
	reqBody any,
	dst any,
	expectedStatus int,
	auth *AuthContext,
) error {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("%s: marshal: %w", op, err)
	}

	req, err := c.newRequest(ctx, op, http.MethodPost, path, body, auth)
	if err != nil {
		return err
	}

	return c.do(op, req, dst, expectedStatus)
}

func (c *ProductClient) ListProducts(ctx context.Context, params *ListProductsParams) (*ProductListResponse, error) {
	q := url.Values{}

	if params.Category != "" {
		q.Set("category", params.Category)
	}
	if params.PriceMin != nil {
		q.Set("price_min", strconv.FormatInt(*params.PriceMin, 10))
	}
	if params.PriceMax != nil {
		q.Set("price_max", strconv.FormatInt(*params.PriceMax, 10))
	}
	if params.Search != "" {
		q.Set("search", params.Search)
	}
	if params.Sort != "" {
		q.Set("sort", params.Sort)
	}
	if params.Cursor != "" {
		q.Set("cursor", params.Cursor)
	}
	if params.Limit > 0 {
		q.Set("limit", strconv.Itoa(params.Limit))
	}

	path := "api/v1/products?" + q.Encode()

	req, err := c.newRequest(ctx, "ListProducts", http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}

	var result ProductListResponse
	if err := c.do("ListProducts", req, &result, http.StatusOK); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *ProductClient) GetProduct(ctx context.Context, id string) (*Product, error) {
	path := fmt.Sprintf("api/v1/products/%s", id)

	req, err := c.newRequest(ctx, "GetProduct", http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}

	var result Product
	if err := c.do("GetProduct", req, &result, http.StatusOK); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *ProductClient) CreateProduct(ctx context.Context, auth *AuthContext, input *CreateProductRequest) (*Product, error) {
	var result Product
	err := c.postJSON(ctx, "CreateProduct", "api/v1/products", input, &result, http.StatusCreated, auth)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *ProductClient) UpdateProduct(ctx context.Context, auth *AuthContext, id string, input *UpdateProductRequest) (*Product, error) {
	path := fmt.Sprintf("api/v1/products/%s", id)

	body, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("UpdateProduct: marshal: %w", err)
	}

	req, err := c.newRequest(ctx, "UpdateProduct", http.MethodPut, path, body, auth)
	if err != nil {
		return nil, err
	}

	var result Product
	if err := c.do("UpdateProduct", req, &result, http.StatusOK); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *ProductClient) DeleteProduct(ctx context.Context, auth *AuthContext, id string) error {
	path := fmt.Sprintf("api/v1/products/%s", id)

	req, err := c.newRequest(ctx, "DeleteProduct", http.MethodDelete, path, nil, auth)
	if err != nil {
		return err
	}

	return c.do("DeleteProduct", req, nil, http.StatusNoContent)
}

func (c *ProductClient) TogglePublish(ctx context.Context, auth *AuthContext, id string) (*Product, error) {
	path := fmt.Sprintf("api/v1/products/%s/publish", id)

	req, err := c.newRequest(ctx, "TogglePublish", http.MethodPost, path, nil, auth)
	if err != nil {
		return nil, err
	}

	var result Product
	if err := c.do("TogglePublish", req, &result, http.StatusOK); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *ProductClient) ListCategories(ctx context.Context) (*CategoryListResponse, error) {
	req, err := c.newRequest(ctx, "ListCategories", http.MethodGet, "api/v1/categories", nil, nil)
	if err != nil {
		return nil, err
	}

	var result CategoryListResponse
	if err := c.do("ListCategories", req, &result, http.StatusOK); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *ProductClient) GetCategory(ctx context.Context, slug string) (*Category, error) {
	path := fmt.Sprintf("api/v1/categories/%s", slug)

	req, err := c.newRequest(ctx, "GetCategory", http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}

	var result Category
	if err := c.do("GetCategory", req, &result, http.StatusOK); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *ProductClient) CreateCategory(ctx context.Context, auth *AuthContext, input *CreateCategoryRequest) (*Category, error) {
	var result Category
	err := c.postJSON(ctx, "CreateCategory", "api/v1/categories", input, &result, http.StatusCreated, auth)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *ProductClient) UpdateCategory(ctx context.Context, auth *AuthContext, slug string, input *UpdateCategoryRequest) (*Category, error) {
	path := fmt.Sprintf("api/v1/categories/%s", slug)

	body, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("UpdateCategory: marshal: %w", err)
	}

	req, err := c.newRequest(ctx, "UpdateCategory", http.MethodPut, path, body, auth)
	if err != nil {
		return nil, err
	}

	var result Category
	if err := c.do("UpdateCategory", req, &result, http.StatusOK); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *ProductClient) DeleteCategory(ctx context.Context, auth *AuthContext, slug string) error {
	path := fmt.Sprintf("api/v1/categories/%s", slug)

	req, err := c.newRequest(ctx, "DeleteCategory", http.MethodDelete, path, nil, auth)
	if err != nil {
		return err
	}

	return c.do("DeleteCategory", req, nil, http.StatusNoContent)
}
