package domain

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID               uuid.UUID  `db:"id"`
	Email            string     `db:"email"`
	PasswordHash     *string    `db:"password_hash"`
	EmailConfirmedAt *time.Time `db:"email_confirmed_at"`
	CreatedAt        time.Time  `db:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at"`
	DeletedAt        *time.Time `db:"deleted_at"`
}
