package model

import "errors"

var (
	ErrBookNotFound  = errors.New("book not found")
	ErrDuplicateISBN = errors.New("duplicate ISBN")
	ErrInvalidBook   = errors.New("invalid book data")
)
