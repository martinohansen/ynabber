package wealthreader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPFailuresDoNotDumpResponses(t *testing.T) {
	for _, status := range []int{401, 429, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
				_, _ = io.WriteString(w, `{"error":"private-response"}`)
			}))
			t.Cleanup(server.Close)
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			client := NewClient(Config{ApiBase: server.URL}, logger)
			_, fetchErr := client.Fetch(context.Background(), Session{})
			auth := NewAuth(Config{OAuthBase: server.URL}, logger)
			_, tokenErr := auth.exchangeCode(context.Background(), pkceChallenge{}, "test-code")
			for endpoint, err := range map[string]error{"fetch": fetchErr, "token": tokenErr} {
				if err == nil || !strings.Contains(err.Error(), fmt.Sprint(status)) || strings.Contains(err.Error(), "private-response") {
					t.Errorf("%s error = %v, want status without response body", endpoint, err)
				}
			}
			if errors.Is(fetchErr, ErrRateLimit) != (status == 429) {
				t.Errorf("incorrect rate-limit classification: %v", fetchErr)
			}
			if errors.Is(fetchErr, ErrUnauthorized) != (status == 401) {
				t.Errorf("incorrect authentication classification: %v", fetchErr)
			}
		})
	}
}

func TestProviderErrorsRetainOnlyCode(t *testing.T) {
	for _, code := range []int{2020, 5000} {
		body := []byte(fmt.Sprintf(`{"success":false,"error":{"code":%d,"message":"private-response"}}`, code))
		_, err := parseResponse(body)
		if err == nil || !strings.Contains(err.Error(), fmt.Sprint(code)) || strings.Contains(err.Error(), "private-response") {
			t.Errorf("error = %v, want code without provider message", err)
		}
		if errors.Is(err, ErrUnauthorized) != (code == 2020) {
			t.Errorf("incorrect authentication classification: %v", err)
		}
	}
}
