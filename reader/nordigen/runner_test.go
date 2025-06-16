package nordigen

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/frieser/nordigen-go-lib/v2"
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
