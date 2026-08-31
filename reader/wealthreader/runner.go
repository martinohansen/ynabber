package wealthreader

import (
	"context"
	"errors"
	"time"

	"github.com/martinohansen/ynabber"
)

const retryBaseDelay = 30 * time.Second

const (
	rateLimitRetryHour   = 6
	rateLimitRetryMinute = 30
)

func (r Reader) after(delay time.Duration) <-chan time.Time {
	if r.afterFn != nil {
		return r.afterFn(delay)
	}
	return time.After(delay)
}

func nextDailyRetryTime(now time.Time) time.Time {
	tomorrow := now.AddDate(0, 0, 1)
	return time.Date(
		tomorrow.Year(), tomorrow.Month(), tomorrow.Day(),
		rateLimitRetryHour, rateLimitRetryMinute, 0, 0,
		tomorrow.Location(),
	)
}

func (r Reader) retryHandler(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrSessionExpired) || errors.Is(err, ErrUnauthorized) {
		return err
	}
	if r.Config.Interval == 0 {
		return err
	}

	var delay time.Duration
	if r.retryDelay != 0 {
		delay = r.retryDelay
		if errors.Is(err, ErrRateLimit) {
			r.logger.Warn("rate limited by API, backing off before retry", "delay", delay)
		} else {
			r.logger.Warn("transient error, backing off before retry", "error", err, "delay", delay)
		}
		select {
		case <-r.after(delay):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if errors.Is(err, ErrRateLimit) {
		retryAt := nextDailyRetryTime(time.Now())
		r.logger.Warn("rate limited by API; will retry at next processing window",
			"retry_at", retryAt.Format(time.RFC3339),
			"wait", time.Until(retryAt).Round(time.Second))
		select {
		case <-r.after(time.Until(retryAt)):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	r.logger.Warn("transient error, backing off before retry", "error", err, "delay", retryBaseDelay)
	select {
	case <-r.after(retryBaseDelay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

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

		batch, err := bulk(ctx)
		if err != nil {
			r.logger.Error("bulk reading transactions", "error", err)
			if err := r.retryHandler(ctx, err); err != nil {
				return err
			}
			continue
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
