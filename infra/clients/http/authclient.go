package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	infracfg "github.com/Krokozabra213/e-commerce_shop/infra/config"
)

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RegisterResponse struct {
	UserID string `json:"user_id"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	UserID       string `json:"user_id"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type RefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type SendVerificationEmailRequest struct {
	UserID string `json:"user_id"`
}

type VerifyEmailResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type GetPublicKeyResponse struct {
	PublicKey string `json:"public_key"`
}

type OAuthLoginResponse struct {
	AuthURL string `json:"auth_url"`
	State   string `json:"state"`
}

type OAuthCallbackResponse struct {
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

func (c *AuthClient) postJSON(ctx context.Context, op, path string, reqBody, dst any, expectedStatus int) error {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("%s: marshal: %w", op, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.Addr+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("%s: new request: %w", op, err)
	}
	req.Header.Set("Content-Type", "application/json")

	return c.do(op, req, dst, expectedStatus)
}

func (c *AuthClient) getJSON(ctx context.Context, op, path string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.config.Addr+path, nil)
	if err != nil {
		return fmt.Errorf("%s: new request: %w", op, err)
	}

	return c.do(op, req, dst, http.StatusOK)
}
func (c *AuthClient) do(op string, req *http.Request, dst any, expectedStatus int) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s: do: %w", op, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != expectedStatus {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s: auth-service returned %d: %s", op, resp.StatusCode, string(respBody))
	}

	if dst == nil || expectedStatus == http.StatusNoContent {
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("%s: decode: %w", op, err)
	}

	return nil
}

func (c *AuthClient) Register(ctx context.Context, email, password string) (*RegisterResponse, error) {
	var result RegisterResponse
	err := c.postJSON(ctx, "Register", "api/v1/auth/register", RegisterRequest{
		Email:    email,
		Password: password,
	}, &result, http.StatusCreated) // 201
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *AuthClient) Login(ctx context.Context, email, password string) (*LoginResponse, error) {
	var result LoginResponse
	err := c.postJSON(ctx, "Login", "api/v1/auth/login", LoginRequest{
		Email:    email,
		Password: password,
	}, &result, http.StatusOK)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *AuthClient) Refresh(ctx context.Context, refreshToken string) (*RefreshResponse, error) {
	var result RefreshResponse
	err := c.postJSON(ctx, "Refresh", "api/v1/auth/refresh", RefreshRequest{
		RefreshToken: refreshToken,
	}, &result, http.StatusOK)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *AuthClient) Logout(ctx context.Context, refreshToken string) error {
	return c.postJSON(ctx, "Logout", "api/v1/auth/logout", LogoutRequest{
		RefreshToken: refreshToken,
	}, nil, http.StatusNoContent)
}

func (c *AuthClient) SendVerificationEmail(ctx context.Context, userID string) error {
	return c.postJSON(ctx, "SendVerificationEmail", "api/v1/auth/resend-verification", SendVerificationEmailRequest{
		UserID: userID,
	}, nil, http.StatusNoContent)
}

func (c *AuthClient) VerifyEmail(ctx context.Context, token string) (*VerifyEmailResponse, error) {
	params := url.Values{}
	params.Set("token", token)
	path := "api/v1/auth/verify-email?" + params.Encode()

	var result VerifyEmailResponse
	if err := c.getJSON(ctx, "VerifyEmail", path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *AuthClient) GetPublicKey(ctx context.Context) (*GetPublicKeyResponse, error) {
	var result GetPublicKeyResponse
	if err := c.getJSON(ctx, "GetPublicKey", "api/v1/auth/public-key", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *AuthClient) OAuthLogin(ctx context.Context, provider string) (*OAuthLoginResponse, error) {
	path := fmt.Sprintf("api/v1/auth/oauth/%s/login", provider)

	var result OAuthLoginResponse
	if err := c.getJSON(ctx, "OAuthLogin", path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *AuthClient) OAuthCallback(ctx context.Context, provider, code, state string) (*OAuthCallbackResponse, error) {
	params := url.Values{}
	params.Set("code", code)
	params.Set("state", state)
	path := fmt.Sprintf("api/v1/auth/oauth/%s/callback?%s", provider, params.Encode())

	var result OAuthCallbackResponse
	if err := c.getJSON(ctx, "OAuthCallback", path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
