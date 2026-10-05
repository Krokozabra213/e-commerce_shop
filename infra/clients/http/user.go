package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	infracfg "github.com/Krokozabra213/e-commerce_shop/infra/config"
	"github.com/Krokozabra213/e-commerce_shop/infra/httpx"
	"github.com/google/uuid"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type UpdateProfileRequest struct {
	FirstName *string `json:"first_name,omitempty"`
	LastName  *string `json:"last_name,omitempty"`
	Phone     *string `json:"phone,omitempty"`
	AvatarURL *string `json:"avatar_url,omitempty"`
}

type UserResponse struct {
	ID        uuid.UUID  `json:"id"`
	Email     string     `json:"email"`
	FirstName *string    `json:"first_name"`
	LastName  *string    `json:"last_name"`
	Phone     *string    `json:"phone"`
	AvatarURL *string    `json:"avatar_url"`
	Roles     []string   `json:"roles"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

type ListUsersResponse struct {
	Users  []UserResponse `json:"users"`
	Total  int            `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

type UserClient struct {
	httpClient *http.Client
	config     infracfg.HTTPClientConfig
}

func NewUserClient(cfg infracfg.HTTPClientConfig) *UserClient {
	addr := strings.TrimRight(cfg.Addr, "/")

	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		addr = "http://" + addr
	}

	cfg.Addr = addr + "/"

	baseTransport := WithRequestID(http.DefaultTransport)
	instrumentedTransport := otelhttp.NewTransport(baseTransport)

	return &UserClient{
		httpClient: &http.Client{
			Timeout:   cfg.Timeout,
			Transport: instrumentedTransport,
		},
		config: cfg,
	}
}

func (c *UserClient) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.config.Addr+"healthz", nil)
	if err != nil {
		return fmt.Errorf("healthcheck: new request: %w", err)
	}

	return c.do("HealthCheck", req, nil, http.StatusOK)
}

func (c *UserClient) ReadyCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.config.Addr+"readyz", nil)
	if err != nil {
		return fmt.Errorf("readycheck: new request: %w", err)
	}

	return c.do("ReadyCheck", req, nil, http.StatusOK)
}

func (c *UserClient) do(op string, req *http.Request, dst any, expectedStatus int) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return apperror.NewInternal(op, err, "Не удалось связаться с user-service", nil)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != expectedStatus {
		return ParseDownstreamError("user-service", resp)
	}

	if dst == nil || expectedStatus == http.StatusNoContent {
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return apperror.NewInternal(op, err, "Не удалось декодировать ответ user-service", nil)
	}

	return nil
}

func (c *UserClient) newRequest(
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
		if len(auth.Roles) > 0 {
			req.Header.Set(httpx.HeaderUserRoles, strings.Join(auth.Roles, ","))
		}
	}

	return req, nil
}

func (c *UserClient) GetMyProfile(ctx context.Context, auth *AuthContext) (*UserResponse, error) {
	req, err := c.newRequest(ctx, "GetMyProfile", http.MethodGet, "api/v1/users/me", nil, auth)
	if err != nil {
		return nil, err
	}

	var result UserResponse
	if err := c.do("GetMyProfile", req, &result, http.StatusOK); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *UserClient) UpdateMyProfile(ctx context.Context, auth *AuthContext, input UpdateProfileRequest) (*UserResponse, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("UpdateMyProfile: marshal: %w", err)
	}

	req, err := c.newRequest(ctx, "UpdateMyProfile", http.MethodPatch, "api/v1/users/me", body, auth)
	if err != nil {
		return nil, err
	}

	var result UserResponse
	if err := c.do("UpdateMyProfile", req, &result, http.StatusOK); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *UserClient) GetUserByID(ctx context.Context, auth *AuthContext, targetID uuid.UUID) (*UserResponse, error) {
	path := fmt.Sprintf("api/v1/users/%s", targetID)

	req, err := c.newRequest(ctx, "GetUserByID", http.MethodGet, path, nil, auth)
	if err != nil {
		return nil, err
	}

	var result UserResponse
	if err := c.do("GetUserByID", req, &result, http.StatusOK); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *UserClient) ListUsers(ctx context.Context, auth *AuthContext, limit, offset int) (*ListUsersResponse, error) {
	params := url.Values{}
	params.Set("limit", fmt.Sprintf("%d", limit))
	params.Set("offset", fmt.Sprintf("%d", offset))
	path := "api/v1/users/?" + params.Encode()

	req, err := c.newRequest(ctx, "ListUsers", http.MethodGet, path, nil, auth)
	if err != nil {
		return nil, err
	}

	var result ListUsersResponse
	if err := c.do("ListUsers", req, &result, http.StatusOK); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *UserClient) DeleteUser(ctx context.Context, auth *AuthContext, targetID uuid.UUID) error {
	path := fmt.Sprintf("api/v1/users/%s", targetID)

	req, err := c.newRequest(ctx, "DeleteUser", http.MethodDelete, path, nil, auth)
	if err != nil {
		return err
	}

	return c.do("DeleteUser", req, nil, http.StatusNoContent)
}

func (c *UserClient) AddRole(ctx context.Context, auth *AuthContext, targetID uuid.UUID, role string) error {
	path := fmt.Sprintf("api/v1/users/%s/roles/%s", targetID, role)

	req, err := c.newRequest(ctx, "AddRole", http.MethodPost, path, nil, auth)
	if err != nil {
		return err
	}

	return c.do("AddRole", req, nil, http.StatusNoContent)
}

func (c *UserClient) RemoveRole(ctx context.Context, auth *AuthContext, targetID uuid.UUID, role string) error {
	path := fmt.Sprintf("api/v1/users/%s/roles/%s", targetID, role)

	req, err := c.newRequest(ctx, "RemoveRole", http.MethodDelete, path, nil, auth)
	if err != nil {
		return err
	}

	return c.do("RemoveRole", req, nil, http.StatusNoContent)
}

type RolesResponse struct {
	Roles []string `json:"roles"`
}

func (c *UserClient) GetMyRoles(ctx context.Context, auth *AuthContext) (*RolesResponse, error) {
	req, err := c.newRequest(ctx, "GetMyRoles", http.MethodGet, "api/v1/users/me/roles", nil, auth)
	if err != nil {
		return nil, err
	}

	var result RolesResponse
	if err := c.do("GetMyRoles", req, &result, http.StatusOK); err != nil {
		return nil, err
	}
	return &result, nil
}
