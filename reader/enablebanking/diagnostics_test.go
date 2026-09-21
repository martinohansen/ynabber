package enablebanking

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTransactionErrorPreservesSafeDiagnostics(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		code   string
	}{
		{"credentials", 401, `{"error":"WRONG_CREDENTIALS_PROVIDED","message":"private-token","detail":{"iban":"private-iban"}}`, "WRONG_CREDENTIALS_PROVIDED"},
		{"configuration", 422, `{"error":"PSU_HEADER_NOT_PROVIDED","detail":"private-token"}`, "PSU_HEADER_NOT_PROVIDED"},
		{"provider", 502, `{"error":"ASPSP_ERROR","message":"private-iban"}`, "ASPSP_ERROR"},
		{"expired session", 401, `{"error":"EXPIRED_SESSION","detail":"private-token"}`, "EXPIRED_SESSION"},
		{"rate limit", 429, `{"error":"ASPSP_RATE_LIMIT_EXCEEDED","detail":"private-token"}`, "ASPSP_RATE_LIMIT_EXCEEDED"},
		{"unknown code", 401, `{"error":"PRIVATE_TOKEN"}`, ""},
		{"free text", 500, `{"error":"private-token\nprivate-iban"}`, ""},
		{"malformed JSON", 502, `{"error":"ASPSP_ERROR",`, ""},
		{"wrong type", 500, `{"error":{"token":"private-token"}}`, ""},
		{"non JSON", 503, `<html>private-token</html>`, ""},
		{"missing code", 401, `{"message":"private-token"}`, ""},
		{"expired code with other status", 403, `{"error":"EXPIRED_SESSION"}`, "EXPIRED_SESSION"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = fmt.Fprint(w, tt.body)
			}))
			t.Cleanup(server.Close)
			client := &Client{BaseURL: server.URL, HTTPClient: server.Client()}
			_, err := client.GetAccountTransactions(context.Background(), "private-token", "account", "2026-01-01", "2026-01-31")
			if err == nil {
				t.Fatal("expected an API error")
			}
			if !strings.Contains(err.Error(), fmt.Sprint(tt.status)) {
				t.Errorf("error omits HTTP status: %v", err)
			}
			if tt.code != "" && !strings.Contains(err.Error(), tt.code) {
				t.Errorf("error omits provider code %q: %v", tt.code, err)
			}
			if tt.code == "" && err.Error() != fmt.Sprintf("API returned status %d", tt.status) {
				t.Errorf("unsafe response should retain only status: %v", err)
			}
			for _, secret := range []string{"private-token", "private-iban", "PRIVATE_TOKEN"} {
				if strings.Contains(err.Error(), secret) {
					t.Errorf("error exposes response data: %v", err)
				}
			}
			if got, want := errors.Is(err, ErrUnauthorized), tt.status == 401 && tt.code == expiredSessionErrorCode; got != want {
				t.Errorf("ErrUnauthorized = %t, want %t", got, want)
			}
			if got, want := errors.Is(err, ErrRateLimit), tt.status == 429; got != want {
				t.Errorf("ErrRateLimit = %t, want %t", got, want)
			}
		})
	}
}
