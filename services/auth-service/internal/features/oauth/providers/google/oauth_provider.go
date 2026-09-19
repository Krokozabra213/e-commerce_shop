package googleProvider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	googleUserInfoURL  = "https://www.googleapis.com/oauth2/v3/userinfo"
	defaultHTTPTimeout = 10 * time.Second
	maxErrorBytes      = 2048
	clockSkew          = 30 * time.Second
	googleIssuer1      = "https://accounts.google.com"
	googleIssuer2      = "accounts.google.com"
)

type GoogleOAuthError struct {
	Op         string
	StatusCode int
	Body       string
	Wrapped    error
}

func (e *GoogleOAuthError) Error() string {
	if e.Wrapped != nil {
		return fmt.Sprintf("google oauth [%s]: %v", e.Op, e.Wrapped)
	}
	return fmt.Sprintf("google oauth [%s]: status %d: %s", e.Op, e.StatusCode, e.Body)
}

func (e *GoogleOAuthError) Unwrap() error { return e.Wrapped }

func gErrWrap(op string, err error) *GoogleOAuthError {
	return &GoogleOAuthError{Op: op, Wrapped: err}
}

func gErrStatus(op string, status int, body string) *GoogleOAuthError {
	return &GoogleOAuthError{Op: op, StatusCode: status, Body: body}
}

type GoogleOption func(*GoogleProvider)

func WithGoogleHTTPClient(c *http.Client) GoogleOption {
	return func(p *GoogleProvider) { p.httpClient = c }
}

func WithGoogleTimeout(d time.Duration) GoogleOption {
	return func(p *GoogleProvider) { p.timeout = d }
}

func WithPromptConsent(enabled bool) GoogleOption {
	return func(p *GoogleProvider) { p.promptConsent = enabled }
}

type GoogleProvider struct {
	config        *oauth2.Config
	httpClient    *http.Client
	timeout       time.Duration
	promptConsent bool
}

func New(clientID, clientSecret, redirectURL string, opts ...GoogleOption) *GoogleProvider {
	p := &GoogleProvider{
		timeout: defaultHTTPTimeout,
	}

	for _, opt := range opts {
		opt(p)
	}

	p.config = &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}

	if p.httpClient == nil {
		p.httpClient = &http.Client{Timeout: p.timeout}
	}

	return p
}

func (p *GoogleProvider) GetAuthURL(state string) string {
	opts := []oauth2.AuthCodeOption{}
	if p.promptConsent {
		opts = append(opts,
			oauth2.AccessTypeOffline,
			oauth2.SetAuthURLParam("prompt", "consent"),
		)
	}
	return p.config.AuthCodeURL(state, opts...)
}

func (p *GoogleProvider) ExchangeCode(ctx context.Context, code string) (*domain.OAuthUserInfo, error) {
	token, err := p.config.Exchange(ctx, code)
	if err != nil {
		return nil, gErrWrap("exchange", err)
	}

	if idToken, ok := token.Extra("id_token").(string); ok && idToken != "" {
		userInfo, err := p.parseIDToken(idToken)
		if err == nil {
			return p.validateUserInfo(userInfo)
		}
	}

	client := p.config.Client(ctx, token)
	userInfo, err := p.fetchUserInfo(ctx, client)
	if err != nil {
		return nil, err
	}

	return p.validateUserInfo(userInfo)
}

type idTokenClaims struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	Iss           string `json:"iss"`
	Aud           string `json:"aud"`
	Exp           int64  `json:"exp"`
}

func (p *GoogleProvider) parseIDToken(idToken string) (*domain.OAuthUserInfo, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid id_token: expected 3 parts, got %d", len(parts))
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}

	var claims idTokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("unmarshal claims: %w", err)
	}

	if claims.Iss != googleIssuer1 && claims.Iss != googleIssuer2 {
		return nil, fmt.Errorf("invalid issuer: %s", claims.Iss)
	}

	if claims.Aud != p.config.ClientID {
		return nil, fmt.Errorf("invalid audience: %s", claims.Aud)
	}

	if time.Now().Unix() > claims.Exp+int64(clockSkew.Seconds()) {
		return nil, fmt.Errorf("id_token expired at %d", claims.Exp)
	}

	return &domain.OAuthUserInfo{
		Provider:       domain.OAuthProviderGoogle,
		ProviderUserID: claims.Sub,
		Email:          claims.Email,
		EmailVerified:  claims.EmailVerified,
		Name:           claims.Name,
		AvatarURL:      claims.Picture,
	}, nil
}

type googleUserInfoResponse struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

func (p *GoogleProvider) fetchUserInfo(ctx context.Context, client *http.Client) (*domain.OAuthUserInfo, error) {
	var info googleUserInfoResponse
	if err := p.doGet(ctx, client, googleUserInfoURL, "fetch_userinfo", &info); err != nil {
		return nil, err
	}

	return &domain.OAuthUserInfo{
		Provider:       domain.OAuthProviderGoogle,
		ProviderUserID: info.Sub,
		Email:          info.Email,
		EmailVerified:  info.EmailVerified,
		Name:           info.Name,
		AvatarURL:      info.Picture,
	}, nil
}

func (p *GoogleProvider) doGet(ctx context.Context, client *http.Client, url, op string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return gErrWrap(op, err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return gErrWrap(op, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBytes))
		return gErrStatus(op, resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return gErrWrap(op, fmt.Errorf("decode response: %w", err))
	}

	return nil
}

func (p *GoogleProvider) validateUserInfo(info *domain.OAuthUserInfo) (*domain.OAuthUserInfo, error) {
	if info.ProviderUserID == "" {
		return nil, domain.ErrEmptyUserID
	}
	if info.Email == "" {
		return nil, domain.ErrEmptyEmail
	}
	if !info.EmailVerified {
		return nil, domain.ErrUnverifiedEmail
	}
	return info, nil
}

func (p *GoogleProvider) Name() domain.OAuthProvider {
	return domain.OAuthProviderGoogle
}

func (p *GoogleProvider) Supports(name domain.OAuthProvider) bool {
	return name == domain.OAuthProviderGoogle
}
