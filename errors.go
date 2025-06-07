package ynabber

import (
	"errors"
	"fmt"
	"time"
)

var ErrNotFound = errors.New("not found")

// RateLimitError means that the request was rate limited
type RateLimitError struct {
	// RetryAfter is the duration to wait before retrying
	RetryAfter time.Duration
}

func (e RateLimitError) Error() string {
	if e.RetryAfter == 0 {
		return "rate limited"
	}
	return fmt.Sprintf("rate limited, retry after %s", e.RetryAfter)
}

// Is checks if the error is a rate limit error
func (e RateLimitError) Is(target error) bool {
	_, ok := target.(RateLimitError)
	return ok
}
