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
	NotFoundError          = errors.New("not found")
	AlreadyExistsError     = errors.New("already exists")
	InsufficientStockError = errors.New("insufficient stock")
)
