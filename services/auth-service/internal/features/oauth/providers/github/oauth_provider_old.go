package githubProvider

//
//import (
//	"context"
//	"encoding/json"
//	"fmt"
//	"io"
//	"net/http"
//	"net/url"
//	"strconv"
//	"strings"
//	"time"
//
//	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
//	"golang.org/x/oauth2"
//	"golang.org/x/oauth2/github"
//)
//
//const (
//	//githubAuthURL      = "https://github.com/login/oauth/authorize"
//	//githubTokenURL     = "https://github.com/login/oauth/access_token"
//	githubUserURL      = "https://api.github.com/user"
//	githubEmailsURL    = "https://api.github.com/user/emails"
//	defaultHTTPTimeout = 10 * time.Second
//
//	githubDefaultScope = "user:email"
//
//	maxErrorBytes = 2048
//)
//
//type Option func(provider *GitHubProvider)
//
////	type GitHubProvider struct {
////		clientID     string
////		clientSecret string
////		redirectURL  string
////		httpClient   *http.Client
////		timeout      time.Duration
////		scopes       []string
////	}
//type GitHubProvider struct {
//	config     *oauth2.Config
//	httpClient *http.Client
//	timeout    time.Duration
//	scopes     []string
//}
//
//func WithScopes(scopes ...string) Option {
//	return func(p *GitHubProvider) {
//		if len(scopes) > 0 {
//			p.scopes = scopes
//		}
//	}
//}
//
//func WithHTTPClient(c *http.Client) Option {
//	return func(p *GitHubProvider) {
//		p.httpClient = c
//	}
//}
//
//func WithTimeout(d time.Duration) Option {
//	return func(p *GitHubProvider) {
//		p.timeout = d
//	}
//}
//
////	func NewGitHubProvider(clientID, clientSecret, redirectURL string, opts ...Option) *GitHubProvider {
////		p := &GitHubProvider{
////			clientID:     clientID,
////			clientSecret: clientSecret,
////			redirectURL:  redirectURL,
////			timeout:      defaultHTTPTimeout,
////			scopes:       []string{githubDefaultScope},
////		}
////
////		for _, opt := range opts {
////			opt(p)
////		}
////
////		if p.httpClient == nil {
////			p.httpClient = &http.Client{
////				Timeout: p.timeout,
////			}
////		}
////
////		return p
////	}
//func NewGitHubProvider(clientID, clientSecret, redirectURL string, opts ...Option) *GitHubProvider {
//	p := &GitHubProvider{
//		timeout: defaultHTTPTimeout,
//		scopes:  []string{githubDefaultScope},
//	}
//
//	for _, opt := range opts {
//		opt(p)
//	}
//
//	p.config = &oauth2.Config{
//		ClientID:     clientID,
//		ClientSecret: clientSecret,
//		RedirectURL:  redirectURL,
//		Scopes:       p.scopes,
//		Endpoint:     github.Endpoint,
//	}
//
//	if p.httpClient == nil {
//		p.httpClient = &http.Client{Timeout: p.timeout}
//	}
//
//	return p
//}
//
//func (p *GitHubProvider) GetAuthURL(state string) string {
//	return p.config.AuthCodeURL(state, oauth2.AccessTypeOnline)
//}
//
////func (p *GitHubProvider) GetAuthURL(state string) string {
////	params := url.Values{
////		"client_id":    {p.clientID},
////		"redirect_uri": {p.redirectURL},
////		"scope":        {strings.Join(p.scopes, " ")},
////		"state":        {state},
////	}
////
////	return githubAuthURL + "?" + params.Encode()
////}
//
////	func (p *GitHubProvider) ExchangeCode(ctx context.Context, code string) (*domain.OAuthUserInfo, error) {
////		accessToken, err := p.exchangeCodeForToken(ctx, code)
////		if err != nil {
////			return nil, fmt.Errorf("exchange code: %w", err)
////		}
////
////		userInfo, err := p.fetchUser(ctx, accessToken)
////		if err != nil {
////			return nil, fmt.Errorf("fetch user: %w", err)
////		}
////
////		if userInfo.Email == "" {
////			emailInfo, err := p.fetchPrimaryEmail(ctx, accessToken)
////			if err != nil {
////				return nil, fmt.Errorf("fetch primary email: %w", err)
////			}
////			userInfo.Email = emailInfo.Email
////			userInfo.EmailVerified = emailInfo.Verified
////		}
////
////		return p.validateUserInfo(userInfo)
////	}
//func (p *GitHubProvider) ExchangeCode(ctx context.Context, code string) (*domain.OAuthUserInfo, error) {
//	token, err := p.config.Exchange(ctx, code)
//	if err != nil {
//		return nil, fmt.Errorf("exchange code: %w", err)
//	}
//
//	client := p.config.Client(ctx, token)
//	userInfo, err := p.fetchUser(ctx, client)
//	if err != nil {
//		return nil, err
//	}
//
//	if userInfo.Email == "" {
//		emailInfo, err := p.fetchPrimaryEmail(ctx, client)
//		if err != nil {
//			return nil, err
//		}
//		userInfo.Email = emailInfo.Email
//		userInfo.EmailVerified = emailInfo.Verified
//	}
//
//	return p.validateUserInfo(userInfo)
//}
//
//func (p *GitHubProvider) doGet(ctx context.Context, client *http.Client, url, op string, out any) error {
//	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
//	if err != nil {
//		return fmt.Errorf("http request: %w", err)
//	}
//
//	setGitHubHeaders(req)
//
//	resp, err := client.Do(req)
//	if err != nil {
//		return errWrap(op, err)
//	}
//	defer func() { _ = resp.Body.Close() }()
//
//	if resp.StatusCode != http.StatusOK {
//		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBytes))
//		return errStatus(op, resp.StatusCode, string(body))
//	}
//
//	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
//		return errWrap(op, fmt.Errorf("decode response: %w", err))
//	}
//
//	return nil
//}
//
//type githubTokenResponse struct {
//	AccessToken string `json:"access_token"`
//	TokenType   string `json:"token_type"`
//	Scope       string `json:"scope"`
//	Error       string `json:"error"`
//	Description string `json:"error_description"`
//}
//
//func (p *GitHubProvider) exchangeCodeForToken(ctx context.Context, code string) (string, error) {
//	data := url.Values{
//		"client_id":     {p.clientID},
//		"client_secret": {p.clientSecret},
//		"code":          {code},
//		"redirect_uri":  {p.redirectURL},
//	}
//
//	req, err := http.NewRequestWithContext(ctx, http.MethodPost, githubTokenURL, strings.NewReader(data.Encode()))
//	if err != nil {
//		return "", fmt.Errorf("create request: %w", err)
//	}
//
//	req.Header.Set("Accept", "application/json")
//	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
//
//	resp, err := p.httpClient.Do(req)
//	if err != nil {
//		return "", fmt.Errorf("http request: %w", err)
//	}
//	defer func() {
//		_ = resp.Body.Close()
//	}()
//
//	if resp.StatusCode != http.StatusOK {
//		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBytes))
//		return "", fmt.Errorf("github returned status %d: %s", resp.StatusCode, string(body))
//	}
//
//	var tokenResp githubTokenResponse
//	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
//		return "", fmt.Errorf("decode response: %w", err)
//	}
//
//	if tokenResp.Error != "" {
//		return "", fmt.Errorf("github oauth error: %s — %s", tokenResp.Error, tokenResp.Description)
//	}
//
//	if tokenResp.AccessToken == "" {
//		return "", fmt.Errorf("github returned empty access_token")
//	}
//
//	return tokenResp.AccessToken, nil
//}
//
//type githubUser struct {
//	ID     int64  `json:"id"`
//	Login  string `json:"login"`
//	Name   string `json:"name"`
//	Email  string `json:"email"`
//	Avatar string `json:"avatar_url"`
//}
//
//func (p *GitHubProvider) fetchUser(ctx context.Context, accessToken string) (*domain.OAuthUserInfo, error) {
//	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubUserURL, nil)
//	if err != nil {
//		return nil, fmt.Errorf("create request: %w", err)
//	}
//	p.setGitHubHeaders(req, accessToken)
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
//		return nil, fmt.Errorf("github returned status %d: %s", resp.StatusCode, string(body))
//	}
//
//	var user githubUser
//	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
//		return nil, fmt.Errorf("decode response: %w", err)
//	}
//
//	return &domain.OAuthUserInfo{
//		Provider:       domain.OAuthProviderGithub,
//		ProviderUserID: strconv.FormatInt(user.ID, 10),
//		Email:          user.Email,
//		EmailVerified:  false,
//		Name:           p.resolveName(user.Name, user.Login),
//		AvatarURL:      user.Avatar,
//	}, nil
//}
//
//type emailInfo struct {
//	Email    string
//	Verified bool
//}
//
//type githubEmail struct {
//	Email    string `json:"email"`
//	Primary  bool   `json:"primary"`
//	Verified bool   `json:"verified"`
//}
//
//func (p *GitHubProvider) fetchPrimaryEmail(ctx context.Context, accessToken string) (*emailInfo, error) {
//	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubEmailsURL, nil)
//	if err != nil {
//		return nil, fmt.Errorf("create request: %w", err)
//	}
//	p.setGitHubHeaders(req, accessToken)
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
//		return nil, fmt.Errorf("github returned status %d: %s", resp.StatusCode, string(body))
//	}
//
//	var emails []githubEmail
//	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
//		return nil, fmt.Errorf("decode response: %w", err)
//	}
//
//	for _, e := range emails {
//		if e.Primary && e.Verified {
//			return &emailInfo{Email: e.Email, Verified: true}, nil
//		}
//	}
//
//	for _, e := range emails {
//		if e.Verified {
//			return &emailInfo{Email: e.Email, Verified: true}, nil
//		}
//	}
//
//	for _, e := range emails {
//		if e.Primary {
//			return &emailInfo{Email: e.Email, Verified: false}, nil
//		}
//	}
//
//	return nil, domain.ErrUnverifiedEmail
//}
//
//func (p *GitHubProvider) setGitHubHeaders(req *http.Request, accessToken string) {
//	req.Header.Set("Authorization", "Bearer "+accessToken)
//	req.Header.Set("Accept", "application/vnd.github+json")
//	req.Header.Set("User-Agent", "MyApp-OAuth/1.0")
//}
//
//func (p *GitHubProvider) resolveName(name, login string) string {
//	if name != "" {
//		return name
//	}
//	return login
//}
//
//func (p *GitHubProvider) validateUserInfo(info *domain.OAuthUserInfo) (*domain.OAuthUserInfo, error) {
//	if info.ProviderUserID == "" {
//		return nil, domain.ErrEmptyUserID
//	}
//
//	if info.Email == "" {
//		return nil, domain.ErrEmptyEmail
//	}
//
//	return info, nil
//}
//
//func (p *GitHubProvider) Name() domain.OAuthProvider {
//	return domain.OAuthProviderGithub
//}
//
//func (p *GitHubProvider) Supports(name domain.OAuthProvider) bool {
//	return name == domain.OAuthProviderGithub
//}
