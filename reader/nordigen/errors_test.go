package nordigen

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/frieser/nordigen-go-lib/v2"
)

func TestAPIResponseErrorPreservesRetryMetadata(t *testing.T) {
	original := &nordigen.RateLimitError{
		APIError:  &nordigen.APIError{StatusCode: 429, Body: "private-response"},
		RateLimit: nordigen.RateLimit{Reset: 42},
	}
	err := apiResponseError(original)
	if got, want := err.Error(), "nordigen API returned status 429"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
	var rateLimit *nordigen.RateLimitError
	if !errors.As(err, &rateLimit) || rateLimit != original || rateLimit.RateLimit.Reset != 42 {
		t.Fatalf("lost rate-limit metadata: %v", err)
	}
	if !errors.Is(err, original) {
		t.Fatalf("lost error identity: %v", err)
	}
	if got := apiResponseError(context.Canceled); got != context.Canceled {
		t.Fatalf("transport error changed: %v", got)
	}
}

type errorTransport func(*http.Request) (*http.Response, error)

func (f errorTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestNewReaderDoesNotDumpTokenResponse(t *testing.T) {
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = errorTransport(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"detail":"private-response"}`)),
		}, nil
	})
	t.Setenv("NORDIGEN_SECRET_ID", "test-id")
	t.Setenv("NORDIGEN_SECRET_KEY", "test-key")
	_, err := NewReader(t.TempDir())
	if err == nil || err.Error() != "initializing nordigen client" {
		t.Fatalf("error = %v, want initialization error without response text", err)
	}
	if errors.Unwrap(err) == nil {
		t.Fatal("lost underlying initialization error")
	}
}
