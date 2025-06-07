package nordigen

import (
	"errors"
	"time"

	"github.com/martinohansen/ynabber"
)

// retryHandler returns err as is unless its retirable in which case it will
// wait until the greater of interval or err can be safely retried.
func (r Reader) retryHandler(err error) error {
	var rl *RateLimitError
	if errors.As(err, &rl) && r.Config.Interval != 0 {
		// If rate limited and not in one-shot mode wait for the greater of
		// RetryAfter or Interval before retrying.
		wait := r.Config.Interval
		if rl.RetryAfter > wait {
			wait = rl.RetryAfter
		}
		r.logger.Info("rate limited, retrying later", "wait", wait)
		time.Sleep(wait)
		return nil
	}
	return err
}

// Runner continuously fetches transactions and sends them to out
func (r Reader) Runner(out chan<- []ynabber.Transaction, errCh chan<- error) {
	for {
		batch, err := r.Bulk()
		if err != nil {
			r.logger.Error("bulk reading transactions", "error", err)
			if err := r.retryHandler(err); err != nil {
				// Error could not be handled, send it to channel
				errCh <- err
				return
			} else {
				// Error has been handled, continue to next run
				continue
			}
		}

		out <- batch

		if r.Config.Interval > 0 {
			r.logger.Info("waiting for next run", "in", r.Config.Interval)
			time.Sleep(r.Config.Interval)
		} else {
			return
		}
	}
}
