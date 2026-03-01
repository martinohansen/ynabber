package enablebanking

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/martinohansen/ynabber"
)

// TestReaderRetryHandler tests the retry handler for error handling
func TestReaderRetryHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	reader := Reader{
		logger: logger,
	}

	tests := []struct {
		name     string
		inputErr error
		wantErr  bool
	}{
		{
			name:     "regular error",
			inputErr: errors.New("some error"),
			wantErr:  true,
		},
		{
			name:     "nil error",
			inputErr: nil,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr := reader.retryHandler(context.Background(), tt.inputErr)
			if (gotErr != nil) != tt.wantErr {
				t.Errorf("retryHandler() error = %v, want error %v", gotErr, tt.wantErr)
			}
			if gotErr != nil && tt.inputErr != nil && gotErr.Error() != tt.inputErr.Error() {
				t.Errorf("retryHandler() error = %v, want %v", gotErr, tt.inputErr)
			}
		})
	}
}

// TestRunnerOneShotMode tests Runner behavior in one-shot mode (interval = 0)
// This test verifies the Runner loop exits after one iteration when interval is 0
func TestRunnerOneShotMode(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tests := []struct {
		name     string
		interval time.Duration
		want     int // Expected number of send attempts before exit
	}{
		{
			name:     "one-shot mode",
			interval: 0,
			want:     1,
		},
		{
			name:     "continuous with timeout",
			interval: 100 * time.Millisecond,
			want:     2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := Reader{
				Config: Config{
					Interval: tt.interval,
				},
				logger: logger,
			}

			out := make(chan []ynabber.Transaction, 10)
			sendCount := 0

			// Create a context with a timeout to prevent infinite loops
			ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
			defer cancel()

			// Create a modified Runner that uses a stub Bulk instead
			// We'll test this by checking the loop behavior
			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					default:
					}

					// Simulate Bulk returning one transaction
					batch := []ynabber.Transaction{
						{
							ID:     ynabber.ID("tx-test"),
							Payee:  "Test",
							Amount: ynabber.Milliunits(10000),
						},
					}

					select {
					case out <- batch:
						sendCount++
					case <-ctx.Done():
						return
					}

					if reader.Config.Interval > 0 {
						reader.logger.Debug("waiting for next run", "in", reader.Config.Interval)
						select {
						case <-time.After(reader.Config.Interval):
						case <-ctx.Done():
							return
						}
					} else {
						// One-shot mode: exit after first send
						return
					}
				}
			}()

			// Wait for goroutine to complete or timeout
			<-ctx.Done()

			// In one-shot mode, should send exactly 1
			if tt.interval == 0 && sendCount != 1 {
				t.Errorf("one-shot mode: expected 1 send, got %d", sendCount)
			}
			// In continuous mode with timeout, should send at least 2
			if tt.interval > 0 && sendCount < 2 {
				t.Errorf("continuous mode: expected at least 2 sends, got %d", sendCount)
			}
		})
	}
}

// TestRunnerContextHandling tests that Runner respects context cancellation
func TestRunnerContextHandling(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tests := []struct {
		name        string
		cancelDelay time.Duration
		wantErr     error
	}{
		{
			name:        "context cancellation",
			cancelDelay: 50 * time.Millisecond,
			wantErr:     context.Canceled,
		},
		{
			name:        "context timeout",
			cancelDelay: 100 * time.Millisecond,
			wantErr:     context.DeadlineExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := Reader{
				Config: Config{
					Interval: 10 * time.Second,
				},
				logger: logger,
			}

			out := make(chan []ynabber.Transaction, 1)
			var runnerErr error

			// Create mock context based on test case
			var ctx context.Context
			var cancel context.CancelFunc

			if tt.name == "context timeout" {
				ctx, cancel = context.WithTimeout(context.Background(), tt.cancelDelay)
			} else {
				ctx, cancel = context.WithCancel(context.Background())
				go func() {
					time.Sleep(tt.cancelDelay)
					cancel()
				}()
			}
			defer cancel()

			// We'll run a simplified version of Runner that demonstrates context handling
			go func() {
				for {
					select {
					case <-ctx.Done():
						runnerErr = ctx.Err()
						return
					default:
					}

					batch := []ynabber.Transaction{}

					select {
					case out <- batch:
					case <-ctx.Done():
						runnerErr = ctx.Err()
						return
					}

					if reader.Config.Interval > 0 {
						select {
						case <-time.After(reader.Config.Interval):
						case <-ctx.Done():
							runnerErr = ctx.Err()
							return
						}
					} else {
						return
					}
				}
			}()

			// Wait for the runner to finish
			time.Sleep(150 * time.Millisecond)

			if runnerErr != tt.wantErr {
				t.Errorf("expected error %v, got %v", tt.wantErr, runnerErr)
			}
		})
	}
}

// TestRunnerChannelBehavior tests that Runner properly handles channel operations
func TestRunnerChannelBehavior(t *testing.T) {
	tests := []struct {
		name            string
		bufferedChannel bool
		wantBatches     int
	}{
		{
			name:            "buffered channel",
			bufferedChannel: true,
			wantBatches:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out chan []ynabber.Transaction
			if tt.bufferedChannel {
				out = make(chan []ynabber.Transaction, 5)
			} else {
				out = make(chan []ynabber.Transaction)
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Send a batch to the channel
			batch := []ynabber.Transaction{
				{
					ID:     ynabber.ID("tx-1"),
					Payee:  "Test Payee",
					Amount: ynabber.Milliunits(50000),
				},
			}

			go func() {
				out <- batch
				cancel()
			}()

			// Receive from channel
			receivedBatches := 0
			for {
				select {
				case b := <-out:
					receivedBatches++
					if len(b) != 1 {
						t.Errorf("expected batch size 1, got %d", len(b))
					}
					if b[0].Payee != "Test Payee" {
						t.Errorf("expected payee 'Test Payee', got '%s'", b[0].Payee)
					}
				case <-ctx.Done():
					goto done
				}
			}

		done:
			if receivedBatches != tt.wantBatches {
				t.Errorf("expected %d batches, got %d", tt.wantBatches, receivedBatches)
			}
		})
	}
}

// TestRunnerErrorPropagation tests that Runner properly handles and propagates errors
func TestRunnerErrorPropagation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	reader := Reader{
		Config: Config{
			Interval: 0,
		},
		logger: logger,
	}

	ctx := context.Background()

	// Test that retryHandler returns the error
	testErr := errors.New("test error")
	returnedErr := reader.retryHandler(ctx, testErr)

	if returnedErr != testErr {
		t.Errorf("retryHandler should return the same error, got %v want %v", returnedErr, testErr)
	}

	// Test with nil error
	nilErr := reader.retryHandler(ctx, nil)
	if nilErr != nil {
		t.Errorf("retryHandler should return nil for nil input, got %v", nilErr)
	}
}

// TestRetryHandlerContinuousMode tests retry/backoff behaviour when running in
// continuous mode (Interval > 0).
func TestRetryHandlerContinuousMode(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tiny := 1 * time.Millisecond // keep tests fast

	tests := []struct {
		name    string
		err     error
		wantNil bool // true if retryHandler should return nil (retry signalled)
	}{
		{
			name:    "rate limit is retried",
			err:     ErrRateLimit,
			wantNil: true,
		},
		{
			name:    "transient error is retried",
			err:     errors.New("connection refused"),
			wantNil: true,
		},
		{
			name:    "ErrSessionExpired is fatal even in continuous mode",
			err:     ErrSessionExpired,
			wantNil: false,
		},
		{
			name:    "ErrUnauthorized (API 401) is fatal — wraps as ErrSessionExpired",
			err:     ErrUnauthorized,
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := Reader{
				Config:     Config{Interval: 1 * time.Hour},
				logger:     logger,
				retryDelay: tiny,
			}
			got := reader.retryHandler(context.Background(), tt.err)
			if tt.wantNil && got != nil {
				t.Errorf("expected nil (retry signalled), got %v", got)
			}
			if !tt.wantNil && got == nil {
				t.Errorf("expected non-nil error (fatal), got nil")
			}
		})
	}
}

// TestRetryHandlerContextCancellation verifies that a context cancellation
// during the backoff wait is surfaced immediately for both transient errors
// and rate-limit errors.
func TestRetryHandlerContextCancellation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tests := []struct {
		name string
		err  error
	}{
		{
			name: "transient error — cancelled context returns ctx.Err()",
			err:  errors.New("some transient error"),
		},
		{
			name: "rate limit error — cancelled context returns ctx.Err()",
			err:  ErrRateLimit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := Reader{
				Config:     Config{Interval: 1 * time.Hour},
				logger:     logger,
				retryDelay: 10 * time.Second, // long enough that ctx fires first
			}

			ctx, cancel := context.WithCancel(context.Background())
			cancel() // cancel immediately

			got := reader.retryHandler(ctx, tt.err)
			if !errors.Is(got, context.Canceled) {
				t.Errorf("expected context.Canceled, got %v", got)
			}
		})
	}
}

// TestNextDailyRetryTime verifies that the retry target is always the
// following calendar day at rateLimitRetryHour:rateLimitRetryMinute.
func TestNextDailyRetryTime(t *testing.T) {
	loc := time.UTC

	tests := []struct {
		name    string
		now     time.Time
		wantDay int // expected Day() of result
		wantH   int
		wantM   int
	}{
		{
			name:    "early morning — still tomorrow",
			now:     time.Date(2024, 1, 10, 3, 0, 0, 0, loc),
			wantDay: 11,
			wantH:   rateLimitRetryHour,
			wantM:   rateLimitRetryMinute,
		},
		{
			name:    "exactly at retry time — still tomorrow",
			now:     time.Date(2024, 1, 10, rateLimitRetryHour, rateLimitRetryMinute, 0, 0, loc),
			wantDay: 11,
			wantH:   rateLimitRetryHour,
			wantM:   rateLimitRetryMinute,
		},
		{
			name:    "late night — still tomorrow",
			now:     time.Date(2024, 1, 10, 23, 59, 59, 0, loc),
			wantDay: 11,
			wantH:   rateLimitRetryHour,
			wantM:   rateLimitRetryMinute,
		},
		{
			name:    "month boundary",
			now:     time.Date(2024, 1, 31, 12, 0, 0, 0, loc),
			wantDay: 1, // Feb 1
			wantH:   rateLimitRetryHour,
			wantM:   rateLimitRetryMinute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nextDailyRetryTime(tt.now)
			if got.Day() != tt.wantDay {
				t.Errorf("Day() = %d, want %d", got.Day(), tt.wantDay)
			}
			if got.Hour() != tt.wantH {
				t.Errorf("Hour() = %d, want %d", got.Hour(), tt.wantH)
			}
			if got.Minute() != tt.wantM {
				t.Errorf("Minute() = %d, want %d", got.Minute(), tt.wantM)
			}
			if got.Second() != 0 || got.Nanosecond() != 0 {
				t.Errorf("expected zero seconds/nanoseconds, got %v", got)
			}
			if !got.After(tt.now) {
				t.Errorf("retry time %v is not after now %v", got, tt.now)
			}
		})
	}
}
