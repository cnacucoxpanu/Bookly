package service

import "errors"

var (
	ErrInvalidInput     = errors.New("invalid input")
	ErrForbidden        = errors.New("forbidden")
	ErrSpaceNotFound    = errors.New("space not found")
	ErrSpaceExists      = errors.New("space already exists")
	ErrBookingNotFound  = errors.New("booking not found")
	ErrSlotTaken        = errors.New("space is already booked for this time")
	ErrAlreadyCancelled = errors.New("booking is already cancelled")
)
