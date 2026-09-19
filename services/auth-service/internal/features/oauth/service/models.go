package oauthservice

import "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"

type GetOAuthLoginURLInput struct {
	Provider domain.OAuthProvider
}

type GetOAuthLoginURLOutput struct {
	AuthURL string
	State   string
}

type OAuthCallbackInput struct {
	Provider domain.OAuthProvider
	Code     string
	State    string
}

type OAuthCallbackOutput struct {
	AccessToken  string
	RefreshToken string
	IsNewUser    bool
	User         *domain.User
}

type GetAvailableProvidersOutput struct {
	Providers []domain.OAuthProvider
}
