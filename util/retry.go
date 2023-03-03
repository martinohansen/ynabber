package util

import (
	"math"
	"time"
)

// Backoff returns a exponential time to wait before making the next retry. The
// wait time will cap at 5 minutes.
func Backoff(attempt int) time.Duration {
	if attempt == 0 {
		return 0
	}
	maxDelay := 5 * time.Minute
	delay := time.Duration(math.Pow(2, float64(attempt))) * 100 * time.Millisecond
	if delay > maxDelay {
		delay = maxDelay
	}
	return delay
}
