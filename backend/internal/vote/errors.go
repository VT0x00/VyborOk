package vote

import "errors"

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrNotFound     = errors.New("not found")
	ErrAuthRequired = errors.New("auth required")
	ErrPollClosed   = errors.New("poll closed")
	ErrAlreadyVoted = errors.New("already voted")
)
