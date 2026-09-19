package domain

type UpdateProfileInput struct {
	FirstName *string
	LastName  *string
	Phone     *string
	AvatarURL *string
}

type ListUsersFilter struct {
	Limit  int
	Offset int
}
