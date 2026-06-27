package model

import "errors"

var (
	ErrBookNotFound              = errors.New("book not found")
	ErrDuplicateISBN             = errors.New("duplicate ISBN")
	ErrInvalidBook               = errors.New("invalid book data")
	ErrNoAvailableCopies         = errors.New("no available copies")
	ErrInvalidAvailability       = errors.New("invalid availability transition")
	ErrBookHasActiveReservations = errors.New("book has active reservations")
)
