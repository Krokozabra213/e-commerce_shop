package domain

import "errors"

var (
	AlreadyExistsError  = errors.New("already exists")
	NotFoundError       = errors.New("not found")
	ErrUnverifiedEmail  = errors.New("email is not verified by provider")
	ErrEmptyUserID      = errors.New("provider returned empty user id")
	ErrEmptyEmail       = errors.New("provider returned empty email")
	ErrProviderNotFound = errors.New("oauth provider not found")
)
