package ynabber

import (
	"errors"
	"testing"
)

func TestErrRateLimitAs(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "rate limit with retry",
			err:  RateLimitError{RetryAfter: 5},
			want: true,
		},
		{
			name: "rate limit without retry",
			err:  RateLimitError{},
			want: true,
		},
		{
			name: "not a rate limit error",
			err:  errors.New("some other error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var e RateLimitError
			if errors.As(tt.err, &e) != tt.want {
				t.Errorf("expected %v, got %v", tt.want, !tt.want)
			}
		})
	}
}
