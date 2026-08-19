package nordigen

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/frieser/nordigen-go-lib/v2"
	"github.com/google/go-cmp/cmp"
	"github.com/martinohansen/ynabber"
)

func TestReaderRetryHandler(t *testing.T) {
	logger := slog.Default()

	rl := &nordigen.RateLimitError{
		APIError: &nordigen.APIError{StatusCode: 429},
		// slightly annoying that this requires us to wait for 1 second until
		// the test case is cached 🤷‍♂️
		RateLimit: nordigen.RateLimit{Reset: 1},
	}

	tests := []struct {
		name     string
		config   Config
		inputErr error
		wantErr  error
	}{
		{
			name:     "not retirable",
			config:   Config{Interval: time.Second},
			inputErr: errors.New("some other error"),
			wantErr:  errors.New("some other error"),
		},
		{
			name:     "no retry in one-shot mode",
			config:   Config{Interval: 0},
			inputErr: rl,
			wantErr:  rl,
		},
		{
			name:     "retry if interval is set",
			config:   Config{Interval: time.Millisecond * 100},
			inputErr: rl,
			wantErr:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := Reader{
				Config: tt.config,
				logger: logger,
				afterFn: func(time.Duration) <-chan time.Time {
					ready := make(chan time.Time, 1)
					ready <- time.Now()
					return ready
				},
			}

			gotErr := reader.retryHandler(context.Background(), tt.inputErr)
			if (gotErr == nil) != (tt.wantErr == nil) {
				t.Errorf("'%v', want '%v'", gotErr, tt.wantErr)
			} else if gotErr != nil && tt.wantErr != nil && gotErr.Error() != tt.wantErr.Error() {
				t.Errorf("'%v', want '%v'", gotErr, tt.wantErr)
			}

		})
	}
}

func TestRunnerOneShotMode(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	want := []ynabber.Transaction{{
		ID:     "tx-test",
		Payee:  "Test",
		Amount: 10000,
	}}
	calls := 0
	reader := Reader{
		Config: Config{Interval: 0},
		logger: logger,
		bulkFn: func() ([]ynabber.Transaction, error) {
			calls++
			return want, nil
		},
	}
	out := make(chan []ynabber.Transaction, 1)

	if err := reader.Runner(context.Background(), out); err != nil {
		t.Fatalf("Runner() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("Bulk calls = %d, want 1", calls)
	}

	select {
	case got := <-out:
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("batch mismatch (-want +got):\n%s", diff)
		}
	default:
		t.Fatal("Runner() did not send a batch")
	}
}

func TestRunnerContinuousMode(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	interval := 5 * time.Minute
	ticks := make(chan time.Time)
	waits := make(chan time.Duration, 2)
	calls := 0
	reader := Reader{
		Config: Config{Interval: interval},
		logger: logger,
		bulkFn: func() ([]ynabber.Transaction, error) {
			calls++
			return []ynabber.Transaction{{ID: ynabber.ID(fmt.Sprintf("tx-%d", calls))}}, nil
		},
		afterFn: func(delay time.Duration) <-chan time.Time {
			waits <- delay
			return ticks
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	out := make(chan []ynabber.Transaction)
	runnerErr := make(chan error, 1)
	go func() {
		runnerErr <- reader.Runner(ctx, out)
	}()

	for wantCall := 1; wantCall <= 2; wantCall++ {
		select {
		case batch := <-out:
			if len(batch) != 1 || batch[0].ID != ynabber.ID(fmt.Sprintf("tx-%d", wantCall)) {
				t.Fatalf("batch %d = %+v", wantCall, batch)
			}
		case <-time.After(time.Second):
			t.Fatalf("Runner() did not send batch %d", wantCall)
		}

		select {
		case got := <-waits:
			if got != interval {
				t.Fatalf("wait after batch %d = %v, want %v", wantCall, got, interval)
			}
		case <-time.After(time.Second):
			t.Fatalf("Runner() did not wait after batch %d", wantCall)
		}

		if wantCall == 1 {
			ticks <- time.Now()
		}
	}

	cancel()
	select {
	case err := <-runnerErr:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Runner() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Runner() did not stop after cancellation")
	}
	if calls != 2 {
		t.Fatalf("Bulk calls = %d, want 2", calls)
	}
}

func TestRunnerCancellationWhileSending(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	fetched := make(chan struct{})
	reader := Reader{
		Config: Config{Interval: 0},
		logger: logger,
		bulkFn: func() ([]ynabber.Transaction, error) {
			close(fetched)
			return []ynabber.Transaction{{ID: "tx-test"}}, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	runnerErr := make(chan error, 1)
	go func() {
		runnerErr <- reader.Runner(ctx, make(chan []ynabber.Transaction))
	}()

	select {
	case <-fetched:
	case <-time.After(time.Second):
		t.Fatal("Runner() did not fetch a batch")
	}
	cancel()

	select {
	case err := <-runnerErr:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Runner() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Runner() remained blocked on output")
	}
}

func TestRunnerErrorPropagation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	wantErr := errors.New("test error")
	reader := Reader{
		Config: Config{Interval: 0},
		logger: logger,
		bulkFn: func() ([]ynabber.Transaction, error) {
			return nil, wantErr
		},
	}

	err := reader.Runner(context.Background(), make(chan []ynabber.Transaction))
	if !errors.Is(err, wantErr) {
		t.Fatalf("Runner() error = %v, want %v", err, wantErr)
	}
}

func TestRunnerRetriesRateLimit(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	interval := time.Hour
	retryDelay := 10 * time.Second
	ticks := make(chan time.Time)
	waits := make(chan time.Duration, 2)
	calls := 0
	rateLimitErr := &nordigen.RateLimitError{
		APIError: &nordigen.APIError{StatusCode: 429},
		RateLimit: nordigen.RateLimit{
			Reset: int(retryDelay/time.Second) - 1,
		},
	}
	reader := Reader{
		Config: Config{Interval: interval},
		logger: logger,
		bulkFn: func() ([]ynabber.Transaction, error) {
			calls++
			if calls == 1 {
				return nil, rateLimitErr
			}
			return []ynabber.Transaction{{ID: "recovered"}}, nil
		},
		afterFn: func(delay time.Duration) <-chan time.Time {
			waits <- delay
			return ticks
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	runnerErr := make(chan error, 1)
	out := make(chan []ynabber.Transaction)
	go func() {
		runnerErr <- reader.Runner(ctx, out)
	}()

	select {
	case got := <-waits:
		if got != retryDelay {
			t.Fatalf("retry wait = %v, want %v", got, retryDelay)
		}
	case <-time.After(time.Second):
		t.Fatal("Runner() did not wait before retrying")
	}
	ticks <- time.Now()

	select {
	case batch := <-out:
		if len(batch) != 1 || batch[0].ID != "recovered" {
			t.Fatalf("recovered batch = %+v", batch)
		}
	case <-time.After(time.Second):
		t.Fatal("Runner() did not send the recovered batch")
	}

	select {
	case got := <-waits:
		if got != interval {
			t.Fatalf("normal wait = %v, want %v", got, interval)
		}
	case <-time.After(time.Second):
		t.Fatal("Runner() did not resume its normal interval")
	}

	cancel()
	select {
	case err := <-runnerErr:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Runner() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Runner() did not stop after cancellation")
	}
	if calls != 2 {
		t.Fatalf("Bulk calls = %d, want 2", calls)
	}
}
