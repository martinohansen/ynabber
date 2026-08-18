package enablebanking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestBulkPropagatesAccountFetchErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantIs     error
	}{
		{
			name:       "unauthorized session",
			statusCode: http.StatusUnauthorized,
			wantIs:     ErrUnauthorized,
		},
		{
			name:       "rate limit",
			statusCode: http.StatusTooManyRequests,
			wantIs:     ErrRateLimit,
		},
		{
			name:       "transient server failure",
			statusCode: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = fmt.Fprintf(w, `{"error":"status %d"}`, tt.statusCode)
			}))
			defer server.Close()

			reader := newBulkTestReader(t, server, []AccountInfo{
				{
					UID:       "account-1",
					AccountID: AccountID{IBAN: "NO9812345678901"},
				},
			})

			transactions, err := reader.Bulk(context.Background())
			if err == nil {
				t.Fatalf("Bulk() error = nil, want HTTP %d error", tt.statusCode)
			}
			if transactions != nil {
				t.Fatalf("Bulk() transactions = %#v, want nil after fetch failure", transactions)
			}
			if tt.wantIs != nil && !errors.Is(err, tt.wantIs) {
				t.Errorf("Bulk() error = %v, want errors.Is(error, %v)", err, tt.wantIs)
			}
			if !strings.Contains(err.Error(), `account "NO98...8901"`) {
				t.Errorf("Bulk() error = %q, want masked account context", err)
			}
		})
	}
}

func TestBulkDiscardsPartialResultsAfterAccountFetchError(t *testing.T) {
	requests := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.Path)
		switch r.URL.Path {
		case "/accounts/account-1/transactions":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{
				"transactions": [{
					"entry_reference": "transaction-1",
					"booking_date": "2024-01-15",
					"credit_debit_indicator": "CRDT",
					"status": "BOOK",
					"transaction_amount": {"currency": "NOK", "amount": "100.00"},
					"remittance_information": ["Test payment"]
				}],
				"pending": []
			}`)
		case "/accounts/account-2/transactions":
			http.Error(w, `{"error":"temporarily unavailable"}`, http.StatusServiceUnavailable)
		default:
			t.Errorf("unexpected request path %q", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	reader := newBulkTestReader(t, server, []AccountInfo{
		{
			UID:       "account-1",
			AccountID: AccountID{IBAN: "NO9812345678901"},
		},
		{
			UID:       "account-2",
			AccountID: AccountID{IBAN: "NO9876543210987"},
		},
	})

	transactions, err := reader.Bulk(context.Background())
	if err == nil {
		t.Fatal("Bulk() error = nil, want second account fetch error")
	}
	if transactions != nil {
		t.Fatalf("Bulk() returned partial transactions %#v, want nil", transactions)
	}
	if len(requests) != 2 {
		t.Fatalf("transaction request count = %d, want 2; paths = %v", len(requests), requests)
	}
	if !strings.Contains(err.Error(), `account "NO98...0987"`) {
		t.Errorf("Bulk() error = %q, want failed account context", err)
	}
}

func newBulkTestReader(t *testing.T, server *httptest.Server, accounts []AccountInfo) Reader {
	t.Helper()

	session := Session{
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Accounts:  accounts,
	}
	sessionData, err := json.Marshal(session)
	if err != nil {
		t.Fatalf("marshaling session: %v", err)
	}
	sessionFile := t.TempDir() + "/session.json"
	if err := os.WriteFile(sessionFile, sessionData, 0600); err != nil {
		t.Fatalf("writing session file: %v", err)
	}

	pemFile := t.TempDir() + "/key.pem"
	if err := os.WriteFile(pemFile, generateTestKeyPair(t), 0600); err != nil {
		t.Fatalf("writing PEM file: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	return Reader{
		Config: Config{
			FromDate: Date(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
			ToDate:   Date(time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)),
		},
		Auth: Auth{
			Config: Config{
				AppID:       "test-app",
				PEMFile:     pemFile,
				SessionFile: sessionFile,
			},
			logger: logger,
		},
		Client: &Client{
			BaseURL:    server.URL,
			HTTPClient: server.Client(),
			logger:     logger,
		},
		logger: logger,
	}
}
