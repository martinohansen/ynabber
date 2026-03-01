package enablebanking

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/martinohansen/ynabber"
)

// retryBaseDelay is the backoff duration for transient (non-rate-limit) errors
// in continuous mode.
const retryBaseDelay = 30 * time.Second

// rateLimitRetryHour and rateLimitRetryMinute define the local time at which we
// retry after hitting a daily API rate limit. Banks typically complete their
// overnight processing around 06:00 local time; 06:30 gives a comfortable
// margin before the first scheduled fetch.
const (
	rateLimitRetryHour   = 6
	rateLimitRetryMinute = 30
)

// nextDailyRetryTime returns the next occurrence of rateLimitRetryHour:rateLimitRetryMinute
// on the calendar day after now. Waiting until then gives the bank's API quota
// time to reset before we try again.
func nextDailyRetryTime(now time.Time) time.Time {
	tomorrow := now.AddDate(0, 0, 1)
	return time.Date(
		tomorrow.Year(), tomorrow.Month(), tomorrow.Day(),
		rateLimitRetryHour, rateLimitRetryMinute, 0, 0,
		tomorrow.Location(),
	)
}

// retryHandler handles errors from Bulk and determines whether retrying is
// appropriate. It returns nil to signal "handled, continue loop" or a non-nil
// error to signal "stop the runner".
//
//   - nil input         → return nil (nothing to handle)
//   - ErrSessionExpired → always fatal; re-authentication is required
//   - Interval == 0     → one-shot mode; never retry, surface error to caller
//   - ErrRateLimit      → wait until 06:30 local time tomorrow (daily quota reset)
//   - other transient   → wait retryBaseDelay then return nil
//   - ctx cancelled     → return ctx.Err() immediately
//
// When r.retryDelay is non-zero it overrides all production waits (tests only).
func (r Reader) retryHandler(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}

	// Session expiry is always fatal — operator must re-authenticate.
	if errors.Is(err, ErrSessionExpired) {
		return err
	}

	// A 401 from the API means the session was revoked or expired server-side.
	// Wrap it as ErrSessionExpired so the operator knows to re-authorize.
	if errors.Is(err, ErrUnauthorized) {
		return fmt.Errorf("%w: %w", ErrSessionExpired, err)
	}

	// One-shot mode: surface error immediately, never block.
	if r.Config.Interval == 0 {
		return err
	}

	var delay time.Duration
	if r.retryDelay != 0 {
		// Test override — skip the wall-clock calculation entirely.
		delay = r.retryDelay
		if errors.Is(err, ErrRateLimit) {
			r.logger.Warn("rate limited by API, backing off before retry", "delay", delay)
		} else {
			r.logger.Warn("transient error, backing off before retry", "error", err, "delay", delay)
		}
		select {
		case <-time.After(delay):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if errors.Is(err, ErrRateLimit) {
		retryAt := nextDailyRetryTime(time.Now())
		r.logger.Warn("rate limited by API; daily quota likely exhausted — will retry at next processing window",
			"retry_at", retryAt.Format(time.RFC3339),
			"wait", time.Until(retryAt).Round(time.Second))
		select {
		case <-time.After(time.Until(retryAt)):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	r.logger.Warn("transient error, backing off before retry", "error", err, "delay", retryBaseDelay)
	select {
	case <-time.After(retryBaseDelay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Runner continuously fetches transactions and sends them to out.
// If Config.Interval is 0, it runs once and returns.
// If Config.Interval > 0, it runs continuously, waiting between fetches.
func (r Reader) Runner(ctx context.Context, out chan<- []ynabber.Transaction) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		batch, err := r.Bulk(ctx)
		if err != nil {
			r.logger.Error("bulk reading transactions", "error", err)
			if err := r.retryHandler(ctx, err); err != nil {
				// Error could not be handled, return it
				return err
			} else {
				// Error has been handled, continue to next run
				continue
			}
		}

		select {
		case out <- batch:
		case <-ctx.Done():
			return ctx.Err()
		}

		if r.Config.Interval > 0 {
			r.logger.Info("waiting for next run", "in", r.Config.Interval)
			select {
			case <-time.After(r.Config.Interval):
			case <-ctx.Done():
				return ctx.Err()
			}
		} else {
			return nil
		}
	}
}
