package domain

import "time"

type Category struct {
	ID          string
	Name        string
	Slug        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type CreateCategoryInput struct {
	Name        string
	Slug        string
	Description string
}

type UpdateCategoryInput struct {
	Name        *string
	Description *string
}
