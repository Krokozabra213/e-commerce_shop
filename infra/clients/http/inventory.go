package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	infracfg "github.com/Krokozabra213/e-commerce_shop/infra/config"
	"github.com/Krokozabra213/e-commerce_shop/infra/httpx"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type CreateStockRequest struct {
	ProductID       string `json:"product_id"`
	InitialQuantity int    `json:"initial_quantity"`
}

type AddStockRequest struct {
	Quantity int `json:"quantity"`
}

type StockResponse struct {
	ProductID         string `json:"product_id"`
	AvailableQuantity int    `json:"available_quantity"`
	InStock           bool   `json:"in_stock"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

type InventoryClient struct {
	httpClient *http.Client
	config     infracfg.HTTPClientConfig
}

func NewInventoryClient(cfg infracfg.HTTPClientConfig) *InventoryClient {
	addr := strings.TrimRight(cfg.Addr, "/")

	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		addr = "http://" + addr
	}

	cfg.Addr = addr + "/"

	baseTransport := WithRequestID(http.DefaultTransport)
	instrumentedTransport := otelhttp.NewTransport(baseTransport)

	return &InventoryClient{
		httpClient: &http.Client{
			Timeout:   cfg.Timeout,
			Transport: instrumentedTransport,
		},
		config: cfg,
	}
}

func (c *InventoryClient) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.config.Addr+"healthz", nil)
	if err != nil {
		return fmt.Errorf("healthcheck: new request: %w", err)
	}

	return c.do("HealthCheck", req, nil, http.StatusOK)
}

func (c *InventoryClient) ReadyCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.config.Addr+"readyz", nil)
	if err != nil {
		return fmt.Errorf("readycheck: new request: %w", err)
	}

	return c.do("ReadyCheck", req, nil, http.StatusOK)
}

func (c *InventoryClient) do(op string, req *http.Request, dst any, expectedStatus int) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return apperror.NewInternal(op, err, "Не удалось связаться с inventory-service", nil)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != expectedStatus {
		return ParseDownstreamError("inventory-service", resp)
	}

	if dst == nil || expectedStatus == http.StatusNoContent {
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return apperror.NewInternal(op, err, "Не удалось декодировать ответ inventory-service", nil)
	}

	return nil
}

func (c *InventoryClient) newRequest(
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

func (c *InventoryClient) GetStock(ctx context.Context, productID string) (*StockResponse, error) {
	path := fmt.Sprintf("api/v1/inventory/%s", productID)

	req, err := c.newRequest(ctx, "GetStock", http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}

	var result StockResponse
	if err := c.do("GetStock", req, &result, http.StatusOK); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *InventoryClient) CreateStock(ctx context.Context, auth *AuthContext, productID string, initialQuantity int) (*MessageResponse, error) {
	body, err := json.Marshal(CreateStockRequest{
		ProductID:       productID,
		InitialQuantity: initialQuantity,
	})
	if err != nil {
		return nil, fmt.Errorf("CreateStock: marshal: %w", err)
	}

	req, err := c.newRequest(ctx, "CreateStock", http.MethodPost, "api/v1/inventory/", body, auth)
	if err != nil {
		return nil, err
	}

	var result MessageResponse
	if err := c.do("CreateStock", req, &result, http.StatusCreated); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *InventoryClient) AddStock(ctx context.Context, auth *AuthContext, productID string, quantity int) (*MessageResponse, error) {
	path := fmt.Sprintf("api/v1/inventory/%s/add", productID)

	body, err := json.Marshal(AddStockRequest{
		Quantity: quantity,
	})
	if err != nil {
		return nil, fmt.Errorf("AddStock: marshal: %w", err)
	}

	req, err := c.newRequest(ctx, "AddStock", http.MethodPost, path, body, auth)
	if err != nil {
		return nil, err
	}

	var result MessageResponse
	if err := c.do("AddStock", req, &result, http.StatusOK); err != nil {
		return nil, err
	}
	return &result, nil
}
