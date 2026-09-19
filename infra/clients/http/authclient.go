package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	infracfg "github.com/Krokozabra213/e-commerce_shop/infra/config"
)

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type RefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type AuthClient struct {
	httpClient *http.Client
	config     infracfg.HTTPClientConfig
}

func NewAuthClient(cfg infracfg.HTTPClientConfig) *AuthClient {
	return &AuthClient{
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		config: cfg,
	}
}

func (c *AuthClient) Refresh(ctx context.Context, refreshToken string) (*RefreshResponse, error) {
	body, err := json.Marshal(RefreshRequest{RefreshToken: refreshToken})
	if err != nil {
		return nil, fmt.Errorf("refresh: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.Addr+"api/v1/auth/refresh", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("refresh: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refresh: do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("refresh: auth-service returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result RefreshResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("refresh: decode: %w", err)
	}

	return &result, nil
}
