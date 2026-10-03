package user

type updateProfileRequest struct {
	FirstName *string `json:"first_name"`
	LastName  *string `json:"last_name"`
	Phone     *string `json:"phone"`
	AvatarURL *string `json:"avatar_url"`
}
