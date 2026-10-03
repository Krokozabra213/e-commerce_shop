package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	infracfg "github.com/Krokozabra213/e-commerce_shop/infra/config"
	"github.com/Krokozabra213/e-commerce_shop/infra/httpx"
	"github.com/google/uuid"
)

type OrderItem struct {
	ProductID string `json:"product_id" validate:"required"`
	Quantity  int    `json:"quantity" validate:"required,min=1"`
}

type CreateOrderRequest struct {
	Items []OrderItem `json:"items"`
}

type CreateOrderResponse struct {
	OrderID    uuid.UUID `json:"order_id"`
	Status     string    `json:"status"`
	TotalPrice int64     `json:"total_price"`
}

type OrderResponse struct {
	OrderID    uuid.UUID `json:"order_id"`
	UserID     uuid.UUID `json:"user_id"`
	Status     string    `json:"status"`
	TotalPrice int64     `json:"total_price"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type PaymentSucceededRequest struct {
	EventID       uuid.UUID `json:"event_id"`
	CorrelationID uuid.UUID `json:"correlation_id"`
}

type PaymentFailedRequest struct {
	EventID       uuid.UUID `json:"event_id"`
	CorrelationID uuid.UUID `json:"correlation_id"`
	Reason        string    `json:"reason"`
}

type OrderClient struct {
	httpClient *http.Client
	config     infracfg.HTTPClientConfig
}

func NewOrderClient(cfg infracfg.HTTPClientConfig) *OrderClient {
	addr := strings.TrimRight(cfg.Addr, "/")

	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		addr = "http://" + addr
	}

	cfg.Addr = addr + "/"
	return &OrderClient{
		httpClient: &http.Client{
			Timeout:   cfg.Timeout,
			Transport: WithRequestID(nil),
		},
		config: cfg,
	}
}

func (c *OrderClient) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.config.Addr+"healthz", nil)
	if err != nil {
		return fmt.Errorf("healthcheck: new request: %w", err)
	}

	return c.do("HealthCheck", req, nil, http.StatusOK)
}

func (c *OrderClient) ReadyCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.config.Addr+"readyz", nil)
	if err != nil {
		return fmt.Errorf("readycheck: new request: %w", err)
	}

	return c.do("ReadyCheck", req, nil, http.StatusOK)
}

func (c *OrderClient) do(op string, req *http.Request, dst any, expectedStatus int) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return apperror.NewInternal(op, err, "Не удалось связаться с order-service", nil)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != expectedStatus {
		return ParseDownstreamError("order-service", resp)
	}

	if dst == nil || expectedStatus == http.StatusNoContent {
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return apperror.NewInternal(op, err, "Не удалось декодировать ответ order-service", nil)
	}

	return nil
}

func (c *OrderClient) newRequest(
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

	req.Header.Set(httpx.HeaderUserID, auth.UserID.String())
	req.Header.Set(httpx.HeaderUserRoles, strings.Join(auth.Roles, ","))

	return req, nil
}

func (c *OrderClient) Create(ctx context.Context, auth *AuthContext, idempotencyKey uuid.UUID, items []OrderItem) (*CreateOrderResponse, error) {
	body, err := json.Marshal(CreateOrderRequest{Items: items})
	if err != nil {
		return nil, fmt.Errorf("create: marshal: %w", err)
	}

	req, err := c.newRequest(ctx, "Create", http.MethodPost, "api/v1/orders/", body, auth)
	if err != nil {
		return nil, err
	}

	req.Header.Set(httpx.HeaderIdempotencyKey, idempotencyKey.String())

	var result CreateOrderResponse
	if err := c.do("Create", req, &result, http.StatusCreated); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *OrderClient) GetByID(ctx context.Context, auth *AuthContext, orderID uuid.UUID) (*OrderResponse, error) {
	path := fmt.Sprintf("api/v1/orders/%s", orderID)

	req, err := c.newRequest(ctx, "GetByID", http.MethodGet, path, nil, auth)
	if err != nil {
		return nil, err
	}

	var result OrderResponse
	if err := c.do("GetByID", req, &result, http.StatusOK); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *OrderClient) GetList(ctx context.Context, auth *AuthContext) ([]OrderResponse, error) {
	req, err := c.newRequest(ctx, "GetList", http.MethodGet, "api/v1/orders/", nil, auth)
	if err != nil {
		return nil, err
	}

	var result []OrderResponse
	if err := c.do("GetList", req, &result, http.StatusOK); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *OrderClient) Ship(ctx context.Context, auth *AuthContext, orderID uuid.UUID) error {
	path := fmt.Sprintf("api/v1/orders/%s/ship", orderID)

	req, err := c.newRequest(ctx, "Ship", http.MethodPost, path, nil, auth)
	if err != nil {
		return err
	}

	return c.do("Ship", req, nil, http.StatusNoContent)
}

func (c *OrderClient) Complete(ctx context.Context, auth *AuthContext, orderID uuid.UUID) error {
	path := fmt.Sprintf("api/v1/orders/%s/complete", orderID)

	req, err := c.newRequest(ctx, "Complete", http.MethodPost, path, nil, auth)
	if err != nil {
		return err
	}

	return c.do("Complete", req, nil, http.StatusNoContent)
}

func (c *OrderClient) PaymentSuccess(ctx context.Context, auth *AuthContext, orderID, eventID, correlationID uuid.UUID) error {
	path := fmt.Sprintf("api/v1/orders/%s/payment/success", orderID)

	body, err := json.Marshal(PaymentSucceededRequest{
		EventID:       eventID,
		CorrelationID: correlationID,
	})
	if err != nil {
		return fmt.Errorf("PaymentSuccess: marshal: %w", err)
	}

	req, err := c.newRequest(ctx, "PaymentSuccess", http.MethodPost, path, body, auth)
	if err != nil {
		return err
	}

	return c.do("PaymentSuccess", req, nil, http.StatusNoContent)
}

func (c *OrderClient) PaymentFailed(ctx context.Context, auth *AuthContext, orderID, eventID, correlationID uuid.UUID, reason string) error {
	path := fmt.Sprintf("api/v1/orders/%s/payment/failed", orderID)

	body, err := json.Marshal(PaymentFailedRequest{
		EventID:       eventID,
		CorrelationID: correlationID,
		Reason:        reason,
	})
	if err != nil {
		return fmt.Errorf("PaymentFailed: marshal: %w", err)
	}

	req, err := c.newRequest(ctx, "PaymentFailed", http.MethodPost, path, body, auth)
	if err != nil {
		return err
	}

	return c.do("PaymentFailed", req, nil, http.StatusNoContent)
}
