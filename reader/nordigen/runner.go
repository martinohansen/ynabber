package nordigen

import (
	"context"
	"errors"
	"time"

	"github.com/frieser/nordigen-go-lib/v2"
	"github.com/martinohansen/ynabber"
)

func (r Reader) after(delay time.Duration) <-chan time.Time {
	if r.afterFn != nil {
		return r.afterFn(delay)
	}
	return time.After(delay)
}

// retryHandler handles rate limit errors by waiting for the reset timer or
// returns err imimediately if it cannot handle the error.
func (r Reader) retryHandler(ctx context.Context, err error) error {
	var rl *nordigen.RateLimitError
	if errors.As(err, &rl) && r.Config.Interval != 0 {
		// Handle rate limit error by waiting until the reset timer expires
		wait := time.Duration(rl.RateLimit.Reset+1) * time.Second
		r.logger.Info("rate limited, retrying later", "wait", wait)

		select {
		case <-r.after(wait):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return err
}

// Runner continuously fetches transactions and sends them to out
func (r Reader) Runner(ctx context.Context, out chan<- []ynabber.Transaction) error {
	bulk := r.Bulk
	if r.bulkFn != nil {
		bulk = r.bulkFn
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		batch, err := bulk()
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
			case <-r.after(r.Config.Interval):
			case <-ctx.Done():
				return ctx.Err()
			}
		} else {
			return nil
		}
	}
}
