package googleProvider

//import (
//	"context"
//	"encoding/base64"
//	"encoding/json"
//	"fmt"
//	"io"
//	"net/http"
//	"net/url"
//	"strings"
//	"time"
//
//	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
//)
//
//const (
//	googleAuthURL     = "https://accounts.google.com/o/oauth2/v2/auth"
//	googleTokenURL    = "https://oauth2.googleapis.com/token"
//	googleUserInfoURL = "https://www.googleapis.com/oauth2/v3/userinfo"
//
//	defaultHTTPTimeout = 10 * time.Second
//	maxErrorBytes      = 2048
//	clockSkew          = 30 * time.Second
//)
//
//type GoogleProvider struct {
//	clientID      string
//	clientSecret  string
//	redirectURL   string
//	httpClient    *http.Client
//	timeout       time.Duration
//	promptConsent bool
//}
//
//type Option func(*GoogleProvider)
//
//func WithHTTPClient(client *http.Client) Option {
//	return func(p *GoogleProvider) {
//		p.httpClient = client
//	}
//}
//
//func WithTimeout(d time.Duration) Option {
//	return func(p *GoogleProvider) {
//		p.httpClient.Timeout = d
//	}
//}
//
//func WithPromptConsent(enabled bool) Option {
//	return func(p *GoogleProvider) {
//		p.promptConsent = enabled
//	}
//}
//
//func New(clientID, clientSecret, redirectURL string, opts ...Option) *GoogleProvider {
//	p := &GoogleProvider{
//		clientID:     clientID,
//		clientSecret: clientSecret,
//		redirectURL:  redirectURL,
//		timeout:      defaultHTTPTimeout,
//	}
//
//	for _, opt := range opts {
//		opt(p)
//	}
//
//	if p.httpClient == nil {
//		p.httpClient = &http.Client{
//			Timeout: p.timeout,
//		}
//	} else {
//		if p.timeout > 0 {
//			p.httpClient.Timeout = p.timeout
//		}
//	}
//
//	return p
//}
//
//func (p *GoogleProvider) GetAuthURL(state string) string {
//	params := url.Values{
//		"client_id":     {p.clientID},
//		"redirect_uri":  {p.redirectURL},
//		"response_type": {"code"},
//		"scope":         {"openid email profile"},
//		"state":         {state},
//	}
//
//	if p.promptConsent {
//		params.Set("access_type", "offline")
//		params.Set("prompt", "consent")
//	}
//
//	return googleAuthURL + "?" + params.Encode()
//}
//
//func (p *GoogleProvider) ExchangeCode(ctx context.Context, code string) (*domain.OAuthUserInfo, error) {
//	tokenResp, err := p.exchangeCodeForToken(ctx, code)
//	if err != nil {
//		return nil, fmt.Errorf("exchange code: %w", err)
//	}
//
//	if tokenResp.IDToken != "" {
//		userInfo, err := p.parseIDToken(tokenResp.IDToken)
//		if err == nil {
//			return p.validateUserInfo(userInfo)
//		}
//	}
//
//	userInfo, err := p.fetchUserInfo(ctx, tokenResp.AccessToken)
//	if err != nil {
//		return nil, fmt.Errorf("fetch user info: %w", err)
//	}
//
//	return p.validateUserInfo(userInfo)
//}
//
//type tokenResponse struct {
//	AccessToken  string `json:"access_token"`
//	RefreshToken string `json:"refresh_token"`
//	ExpiresIn    int    `json:"expires_in"`
//	TokenType    string `json:"token_type"`
//	IDToken      string `json:"id_token"`
//}
//
//func (p *GoogleProvider) exchangeCodeForToken(ctx context.Context, code string) (*tokenResponse, error) {
//	data := url.Values{
//		"code":          {code},
//		"client_id":     {p.clientID},
//		"client_secret": {p.clientSecret},
//		"redirect_uri":  {p.redirectURL},
//		"grant_type":    {"authorization_code"},
//	}
//
//	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenURL, strings.NewReader(data.Encode()))
//	if err != nil {
//		return nil, fmt.Errorf("create request: %w", err)
//	}
//	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
//
//	resp, err := p.httpClient.Do(req)
//	if err != nil {
//		return nil, fmt.Errorf("http request: %w", err)
//	}
//	defer func() {
//		_ = resp.Body.Close()
//	}()
//
//	if resp.StatusCode != http.StatusOK {
//		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBytes))
//		return nil, fmt.Errorf("google returned status %d: %s", resp.StatusCode, string(body))
//	}
//
//	var tokenResp tokenResponse
//	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
//		return nil, fmt.Errorf("decode response: %w", err)
//	}
//
//	return &tokenResp, nil
//}
//
//type idTokenClaims struct {
//	Sub           string `json:"sub"`
//	Email         string `json:"email"`
//	EmailVerified bool   `json:"email_verified"`
//	Name          string `json:"name"`
//	Picture       string `json:"picture"`
//	Iss           string `json:"iss"`
//	Aud           string `json:"aud"`
//	Exp           int64  `json:"exp"`
//	Iat           int64  `json:"iat"`
//}
//
//func (p *GoogleProvider) parseIDToken(idToken string) (*domain.OAuthUserInfo, error) {
//	parts := strings.Split(idToken, ".")
//	if len(parts) != 3 {
//		return nil, fmt.Errorf("invalid id_token format: expected 3 parts, got %d", len(parts))
//	}
//
//	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
//	if err != nil {
//		return nil, fmt.Errorf("decode id_token payload: %w", err)
//	}
//
//	var claims idTokenClaims
//	if err := json.Unmarshal(payload, &claims); err != nil {
//		return nil, fmt.Errorf("unmarshal id_token claims: %w", err)
//	}
//
//	if claims.Iss != "https://accounts.google.com" && claims.Iss != "accounts.google.com" {
//		return nil, fmt.Errorf("invalid id_token issuer: %s", claims.Iss)
//	}
//
//	if claims.Aud != p.clientID {
//		return nil, fmt.Errorf("invalid id_token audience: %s", claims.Aud)
//	}
//
//	if time.Now().Unix() > claims.Exp+int64(clockSkew.Seconds()) {
//		return nil, fmt.Errorf("id_token expired at %d", claims.Exp)
//	}
//
//	return &domain.OAuthUserInfo{
//		Provider:       domain.OAuthProviderGoogle,
//		ProviderUserID: claims.Sub,
//		Email:          claims.Email,
//		EmailVerified:  claims.EmailVerified,
//		Name:           claims.Name,
//		AvatarURL:      claims.Picture,
//	}, nil
//}
//
//type userInfoResponse struct {
//	Sub           string `json:"sub"`
//	Email         string `json:"email"`
//	EmailVerified bool   `json:"email_verified"`
//	Name          string `json:"name"`
//	Picture       string `json:"picture"`
//}
//
//func (p *GoogleProvider) fetchUserInfo(ctx context.Context, accessToken string) (*domain.OAuthUserInfo, error) {
//	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleUserInfoURL, nil)
//	if err != nil {
//		return nil, fmt.Errorf("create request: %w", err)
//	}
//	req.Header.Set("Authorization", "Bearer "+accessToken)
//
//	resp, err := p.httpClient.Do(req)
//	if err != nil {
//		return nil, fmt.Errorf("http request: %w", err)
//	}
//	defer func() {
//		_ = resp.Body.Close()
//	}()
//
//	if resp.StatusCode != http.StatusOK {
//		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBytes))
//		return nil, fmt.Errorf("google returned status %d: %s", resp.StatusCode, string(body))
//	}
//
//	var info userInfoResponse
//	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
//		return nil, fmt.Errorf("decode response: %w", err)
//	}
//
//	return &domain.OAuthUserInfo{
//		Provider:       domain.OAuthProviderGoogle,
//		ProviderUserID: info.Sub,
//		Email:          info.Email,
//		EmailVerified:  info.EmailVerified,
//		Name:           info.Name,
//		AvatarURL:      info.Picture,
//	}, nil
//}
//
//func (p *GoogleProvider) validateUserInfo(info *domain.OAuthUserInfo) (*domain.OAuthUserInfo, error) {
//	if info.ProviderUserID == "" {
//		return nil, domain.ErrEmptyUserID
//	}
//
//	if info.Email == "" {
//		return nil, domain.ErrEmptyEmail
//	}
//
//	if !info.EmailVerified {
//		return nil, domain.ErrUnverifiedEmail
//	}
//
//	return info, nil
//}
//
//func (p *GoogleProvider) Name() domain.OAuthProvider {
//	return domain.OAuthProviderGoogle
//}
//
//func (p *GoogleProvider) Supports(name domain.OAuthProvider) bool {
//	return name == domain.OAuthProviderGoogle
//}
