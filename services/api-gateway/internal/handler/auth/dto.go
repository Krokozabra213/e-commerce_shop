package auth

type registerRequest struct {
	Email    string `json:"email" validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

type loginRequest struct {
	Email    string `json:"email" validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
}

type loginResponse struct {
	AccessToken string `json:"access_token"`
	UserID      string `json:"user_id"`
}
