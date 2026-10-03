package handler

import (
	"time"

	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/domain"
	"github.com/google/uuid"
)

type UpdateProfileRequest struct {
	FirstName *string `json:"first_name"`
	LastName  *string `json:"last_name"`
	Phone     *string `json:"phone"`
	AvatarURL *string `json:"avatar_url"`
}

func (r UpdateProfileRequest) ToDomain() domain.UpdateProfileInput {
	return domain.UpdateProfileInput{
		FirstName: r.FirstName,
		LastName:  r.LastName,
		Phone:     r.Phone,
		AvatarURL: r.AvatarURL,
	}
}

type UserResponse struct {
	ID        uuid.UUID  `json:"id"`
	Email     string     `json:"email"`
	FirstName *string    `json:"first_name"`
	LastName  *string    `json:"last_name"`
	Phone     *string    `json:"phone"`
	AvatarURL *string    `json:"avatar_url"`
	Roles     []string   `json:"roles"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

func NewUserResponse(u *domain.User) UserResponse {
	roles := make([]string, len(u.Roles))
	for i, r := range u.Roles {
		roles[i] = string(r)
	}

	return UserResponse{
		ID:        u.ID,
		Email:     u.Email,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Phone:     u.Phone,
		AvatarURL: u.AvatarURL,
		Roles:     roles,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
		DeletedAt: u.DeletedAt,
	}
}

type ListUsersResponse struct {
	Users  []UserResponse `json:"users"`
	Total  int            `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

type RolesResponse struct {
	Roles []string `json:"roles"`
}
