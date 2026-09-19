package authservice

import "github.com/google/uuid"

type RegisterInput struct {
	Email    string
	Password string
}

type RegisterOutput struct {
	UserID uuid.UUID
}

type LoginInput struct {
	Email    string
	Password string
}

type LoginOutput struct {
	AccessToken  string
	RefreshToken string
	UserID       uuid.UUID
}

type RefreshInput struct {
	RefreshToken string
}

type RefreshOutput struct {
	AccessToken  string
	RefreshToken string
}

type LogoutInput struct {
	RefreshToken string
}

type SendVerificationEmailInput struct {
	UserID uuid.UUID
}

type VerifyEmailInput struct {
	Token string
}

type GetPublicKeyOutput struct {
	PublicKey string
}
