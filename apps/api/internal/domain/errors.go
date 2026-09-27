package domain

import "errors"

var (
	// ErrNotFound is returned when a requested resource does not exist.
	ErrNotFound = errors.New("resource not found")

	// ErrConflict is returned when a unique constraint is violated.
	ErrConflict = errors.New("resource already exists")

	// ErrInvalidURL is returned when the provided URL is malformed or not an allowed scheme.
	ErrInvalidURL = errors.New("invalid target url")
)
