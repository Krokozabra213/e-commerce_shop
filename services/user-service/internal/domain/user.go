package domain

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleUser    Role = "ROLE_USER"
	RoleManager Role = "ROLE_MANAGER"
	RoleAdmin   Role = "ROLE_ADMIN"
)

func (r Role) IsValid() bool {
	switch r {
	case RoleUser, RoleManager, RoleAdmin:
		return true
	}
	return false
}

func (r Role) Weight() int {
	switch r {
	case RoleUser:
		return 1
	case RoleManager:
		return 2
	case RoleAdmin:
		return 3
	default:
		return 0
	}
}

func (r Role) String() string {
	return string(r)
}

type User struct {
	ID        uuid.UUID  `db:"id"`
	Email     string     `db:"email"`
	FirstName *string    `db:"first_name"`
	LastName  *string    `db:"last_name"`
	Phone     *string    `db:"phone"`
	AvatarURL *string    `db:"avatar_url"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`

	Roles []Role `db:"-"`
}
