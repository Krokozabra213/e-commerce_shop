package githubProvider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

const (
	githubUserURL      = "https://api.github.com/user"
	githubEmailsURL    = "https://api.github.com/user/emails"
	defaultHTTPTimeout = 10 * time.Second
	githubDefaultScope = "user:email"
	maxErrorBytes      = 2048
	userAgent          = "MyApp-OAuth/1.0"
)

type OAuthError struct {
	Op         string
	StatusCode int
	Body       string
	Wrapped    error
}

func (e *OAuthError) Error() string {
	if e.Wrapped != nil {
		return fmt.Sprintf("github oauth [%s]: %v", e.Op, e.Wrapped)
	}
	return fmt.Sprintf("github oauth [%s]: status %d: %s", e.Op, e.StatusCode, e.Body)
}

func (e *OAuthError) Unwrap() error { return e.Wrapped }

func errWrap(op string, err error) *OAuthError {
	return &OAuthError{Op: op, Wrapped: err}
}

func errStatus(op string, status int, body string) *OAuthError {
	return &OAuthError{Op: op, StatusCode: status, Body: body}
}

type Option func(*GitHubProvider)

func WithScopes(scopes ...string) Option {
	return func(p *GitHubProvider) {
		if len(scopes) > 0 {
			p.scopes = scopes
		}
	}
}

func WithHTTPClient(c *http.Client) Option {
	return func(p *GitHubProvider) {
		p.httpClient = c
	}
}

func WithTimeout(d time.Duration) Option {
	return func(p *GitHubProvider) {
		p.timeout = d
	}
}

type GitHubProvider struct {
	config     *oauth2.Config
	httpClient *http.Client
	timeout    time.Duration
	scopes     []string
}

func New(clientID, clientSecret, redirectURL string, opts ...Option) *GitHubProvider {
	p := &GitHubProvider{
		timeout: defaultHTTPTimeout,
		scopes:  []string{githubDefaultScope},
	}

	for _, opt := range opts {
		opt(p)
	}

	p.config = &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       p.scopes,
		Endpoint:     github.Endpoint,
	}

	if p.httpClient == nil {
		p.httpClient = &http.Client{Timeout: p.timeout}
	}

	return p
}

func (p *GitHubProvider) GetAuthURL(state string) string {
	return p.config.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

func (p *GitHubProvider) ExchangeCode(ctx context.Context, code string) (*domain.OAuthUserInfo, error) {
	token, err := p.config.Exchange(ctx, code)
	if err != nil {
		return nil, errWrap("exchange", err)
	}

	client := p.config.Client(ctx, token)
	userInfo, err := p.fetchUser(ctx, client)
	if err != nil {
		return nil, err
	}

	if userInfo.Email == "" {
		emailInfo, err := p.fetchPrimaryEmail(ctx, client)
		if err != nil {
			return nil, err
		}
		userInfo.Email = emailInfo.Email
		userInfo.EmailVerified = emailInfo.Verified
	}

	return p.validateUserInfo(userInfo)
}

func (p *GitHubProvider) doGet(ctx context.Context, client *http.Client, url, op string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return errWrap(op, err)
	}

	setGitHubHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		return errWrap(op, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBytes))
		return errStatus(op, resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return errWrap(op, fmt.Errorf("decode response: %w", err))
	}

	return nil
}

type githubUser struct {
	ID     int64  `json:"id"`
	Login  string `json:"login"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Avatar string `json:"avatar_url"`
}

func (p *GitHubProvider) fetchUser(ctx context.Context, client *http.Client) (*domain.OAuthUserInfo, error) {
	var user githubUser
	if err := p.doGet(ctx, client, githubUserURL, "fetch_user", &user); err != nil {
		return nil, err
	}

	return &domain.OAuthUserInfo{
		Provider:       domain.OAuthProviderGithub,
		ProviderUserID: strconv.FormatInt(user.ID, 10),
		Email:          user.Email,
		EmailVerified:  false,
		Name:           resolveName(user.Name, user.Login),
		AvatarURL:      user.Avatar,
	}, nil
}

type githubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

type emailInfo struct {
	Email    string
	Verified bool
}

func (p *GitHubProvider) fetchPrimaryEmail(ctx context.Context, client *http.Client) (*emailInfo, error) {
	var emails []githubEmail
	if err := p.doGet(ctx, client, githubEmailsURL, "fetch_email", &emails); err != nil {
		return nil, err
	}

	primaryVerified := pickEmail(emails, func(e githubEmail) bool { return e.Primary && e.Verified })
	if primaryVerified != nil {
		return &emailInfo{Email: primaryVerified.Email, Verified: true}, nil
	}

	anyVerified := pickEmail(emails, func(e githubEmail) bool { return e.Verified })
	if anyVerified != nil {
		return &emailInfo{Email: anyVerified.Email, Verified: true}, nil
	}

	anyPrimary := pickEmail(emails, func(e githubEmail) bool { return e.Primary })
	if anyPrimary != nil {
		return &emailInfo{Email: anyPrimary.Email, Verified: false}, nil
	}

	return nil, domain.ErrUnverifiedEmail
}

func pickEmail(emails []githubEmail, match func(githubEmail) bool) *githubEmail {
	for i := range emails {
		if match(emails[i]) {
			return &emails[i]
		}
	}
	return nil
}

func (p *GitHubProvider) validateUserInfo(info *domain.OAuthUserInfo) (*domain.OAuthUserInfo, error) {
	if info.ProviderUserID == "" {
		return nil, domain.ErrEmptyUserID
	}
	if info.Email == "" {
		return nil, domain.ErrEmptyEmail
	}
	return info, nil
}

func setGitHubHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", userAgent)
}

func resolveName(name, login string) string {
	if name != "" {
		return name
	}
	return login
}

func (p *GitHubProvider) Name() domain.OAuthProvider {
	return domain.OAuthProviderGithub
}

func (p *GitHubProvider) Supports(name domain.OAuthProvider) bool {
	return name == domain.OAuthProviderGithub
}
