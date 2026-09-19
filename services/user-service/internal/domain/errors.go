package domain

import "errors"

var (
	AlreadyExistsError = errors.New("already exists")
	NotFoundError      = errors.New("not found")
)
