package domain

import "errors"

type ReservationFailedReason string

const (
	ReservationFailedReasonNotFound          ReservationFailedReason = "NOT_FOUND"
	ReservationFailedReasonInternal          ReservationFailedReason = "INTERNAL"
	ReservationFailedReasonInsufficientStock ReservationFailedReason = "INSUFFICIENT_STOCK"
)

func (r ReservationFailedReason) String() string {
	return string(r)
}

var (
	ErrNotFound          = errors.New("not found")
	ErrAlreadyExists     = errors.New("already exists")
	ErrInsufficientStock = errors.New("insufficient stock")
)
