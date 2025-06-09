package nordigen

import (
	"context"
	"errors"
	"time"

	"github.com/martinohansen/ynabber"
)

// retryHandler returns err as is unless its retirable in which case it will
// wait until the greater of interval or err can be safely retried.
func (r Reader) retryHandler(ctx context.Context, err error) error {
	var rl *RateLimitError
	if errors.As(err, &rl) && r.Config.Interval != 0 {
		// If rate limited and not in one-shot mode wait for the greater of
		// RetryAfter or Interval before retrying.
		wait := r.Config.Interval
		if rl.RetryAfter > wait {
			wait = rl.RetryAfter
		}
		r.logger.Info("rate limited, retrying later", "wait", wait)

		select {
		case <-time.After(wait):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return err
}

// Runner continuously fetches transactions and sends them to out
func (r Reader) Runner(ctx context.Context, out chan<- []ynabber.Transaction) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		batch, err := r.Bulk()
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
