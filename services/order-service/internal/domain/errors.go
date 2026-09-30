package domain

import "errors"

var (
	ErrNotFound           = errors.New("not found")
	ErrAlreadyExists      = errors.New("already exists")
	ErrMismatchOrNotFound = errors.New("mismatch or not found")
)
