package domain

import (
	"time"

	"github.com/google/uuid"
)

type OauthAccount struct {
	ID             uuid.UUID `db:"id"`
	UserID         uuid.UUID `db:"user_id"`
	Provider       string    `db:"provider"`
	ProviderUserID string    `db:"provider_user_id"`
	CreatedAt      time.Time `db:"created_at"`
}

type OAuthProvider string

const (
	OAuthProviderGoogle OAuthProvider = "google"
	OAuthProviderGithub OAuthProvider = "github"
)

func (p OAuthProvider) String() string {
	return string(p)
}

func (p OAuthProvider) IsValid() bool {
	return p == OAuthProviderGoogle || p == OAuthProviderGithub
}

type OAuthUserInfo struct {
	Provider       OAuthProvider
	ProviderUserID string
	Email          string
	EmailVerified  bool
	Name           string
	AvatarURL      string
}
