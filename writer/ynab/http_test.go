package ynab

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/martinohansen/ynabber"
)

func TestBulkHTTPContract(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	transactionDate := time.Date(now.Year(), now.Month(), now.Day()-1, 12, 0, 0, 0, time.UTC)
	source := ynabber.Transaction{
		Account: ynabber.Account{IBAN: "DK5000400440116243"},
		ID:      "bank-transaction-42",
		Date:    transactionDate,
		Payee:   "Coffee Shop",
		Memo:    "Morning coffee",
		Amount:  -12340,
	}

	requestChecked := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		defer func() { requestChecked <- struct{}{} }()

		if request.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", request.Method, http.MethodPost)
		}
		wantPath := "/v1/budgets/budget%2Fwith%20space/transactions"
		if request.URL.EscapedPath() != wantPath {
			t.Errorf("escaped path = %q, want %q", request.URL.EscapedPath(), wantPath)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer test-token")
		}
		if got := request.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want %q", got, "application/json")
		}

		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("reading request body: %v", err)
			return
		}
		wantBody := fmt.Sprintf(
			`{"transactions":[{"account_id":"ynab-account-id","date":"%s","amount":"-12340","payee_name":"Coffee Shop","memo":"Morning coffee","import_id":"%s","cleared":"reconciled","approved":false}]}`,
			transactionDate.Format(dateFormat),
			makeID(source),
		)
		if string(body) != wantBody {
			t.Errorf("request body = %s, want %s", body, wantBody)
		}

		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(http.StatusCreated)
		_, _ = response.Write([]byte(`{"data":{"transaction_ids":["created-id"]}}`))
	}))
	t.Cleanup(server.Close)

	writer := Writer{
		Config: Config{
			BudgetID:   "budget/with space",
			Token:      "test-token",
			AccountMap: AccountMap{"DK5000400440116243": "ynab-account-id"},
			Cleared:    Reconciled,
		},
		logger:  slog.Default(),
		client:  server.Client(),
		baseURL: server.URL + "/v1/",
	}

	if err := writer.Bulk(context.Background(), []ynabber.Transaction{source}); err != nil {
		t.Fatalf("Bulk() error = %v", err)
	}

	select {
	case <-requestChecked:
	case <-time.After(time.Second):
		t.Fatal("server did not receive request")
	}
}

func TestBulkReturnsAPIError(t *testing.T) {
	t.Parallel()

	for _, status := range []int{
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusUnprocessableEntity,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
	} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				response.WriteHeader(status)
				_, _ = response.Write([]byte(`{"error":"response must not leak"}`))
			}))
			t.Cleanup(server.Close)

			writer, source := testHTTPWriter(server.Client(), server.URL)
			err := writer.Bulk(context.Background(), []ynabber.Transaction{source})
			if err == nil {
				t.Fatal("Bulk() error = nil, want API error")
			}
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("Bulk() error = %T, want *APIError", err)
			}
			if apiErr.StatusCode != status || strings.Contains(err.Error(), "response must not leak") {
				t.Errorf("Bulk() error = %q, status = %d", err, apiErr.StatusCode)
			}
		})
	}
}

func TestBulkReturnsHTTPClientError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("transport unavailable")
	writer, source := testHTTPWriter(errorHTTPClient{err: wantErr}, "https://ynab.invalid/v1")
	err := writer.Bulk(context.Background(), []ynabber.Transaction{source})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Bulk() error = %v, want %v", err, wantErr)
	}
}

func TestBulkPreservesTransportErrorAfterCancellation(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("transport failed")
	writer, source := testHTTPWriter(errorHTTPClient{err: wantErr}, "https://ynab.invalid/v1")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := writer.Bulk(ctx, []ynabber.Transaction{source}); !errors.Is(err, wantErr) {
		t.Fatalf("Bulk() error = %v, want transport error %v", err, wantErr)
	}
}

func TestBulkUsesProductionHTTPDefaults(t *testing.T) {
	t.Parallel()

	writer, source := testHTTPWriter(nil, "")
	client := &recordingHTTPClient{
		response: &http.Response{
			StatusCode: http.StatusCreated,
			Status:     "201 Created",
			Body:       io.NopCloser(strings.NewReader(`{"data":{}}`)),
		},
	}
	writer.client = client

	if err := writer.Bulk(context.Background(), []ynabber.Transaction{source}); err != nil {
		t.Fatalf("Bulk() error = %v", err)
	}
	if got, want := client.request.URL.String(), defaultBaseURL+"/budgets/budget-id/transactions"; got != want {
		t.Errorf("request URL = %q, want %q", got, want)
	}
}

func TestNewWriterUsesExplicitDependencies(t *testing.T) {
	t.Parallel()
	client := &http.Client{Timeout: time.Second}
	config := Config{
		BudgetID:   "budget",
		Token:      "token",
		AccountMap: AccountMap{"bank": "ynab"},
	}
	writer, err := NewWriter(config, WriterOptions{
		Logger: slog.Default(), HTTPClient: client, BaseURL: "https://ynab.example/v1/",
	})
	if err != nil {
		t.Fatal(err)
	}
	if writer.client != client || writer.baseURL != "https://ynab.example/v1" {
		t.Fatalf("writer dependencies were not retained")
	}
	if writer.Config.Cleared != Cleared {
		t.Fatalf("Cleared = %q, want %q", writer.Config.Cleared, Cleared)
	}
}

func TestNewWriterUsesRuntimeDefaults(t *testing.T) {
	t.Parallel()

	config := Config{
		BudgetID:   "budget",
		Token:      "token",
		AccountMap: AccountMap{"bank": "ynab"},
	}
	writer, err := NewWriter(config, WriterOptions{})
	if err != nil {
		t.Fatal(err)
	}
	client, ok := writer.client.(*http.Client)
	if !ok || client.Timeout != 30*time.Second {
		t.Fatalf("HTTP client = %#v, want 30 second default timeout", writer.client)
	}
	if writer.logger == nil || writer.baseURL != defaultBaseURL {
		t.Fatal("NewWriter() did not apply logger or base URL defaults")
	}
}

func TestNewWriterValidatesConfig(t *testing.T) {
	t.Parallel()

	valid := Config{
		BudgetID:   "budget",
		Token:      "token",
		AccountMap: AccountMap{"bank": "ynab"},
	}
	tests := []struct {
		name   string
		change func(*Config)
	}{
		{name: "budget ID", change: func(config *Config) { config.BudgetID = " " }},
		{name: "token", change: func(config *Config) { config.Token = " " }},
		{name: "account map", change: func(config *Config) { config.AccountMap = nil }},
		{name: "transaction status", change: func(config *Config) { config.Cleared = "invalid" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := valid
			test.change(&config)
			if _, err := NewWriter(config, WriterOptions{}); err == nil {
				t.Fatalf("NewWriter() accepted invalid %s", test.name)
			}
		})
	}
}

func TestNewWriterFromEnv(t *testing.T) {
	tests := []struct {
		name       string
		clearedEnv string
		want       TransactionStatus
	}{
		{name: "configured status", clearedEnv: "reconciled", want: Reconciled},
		{name: "default status", want: Cleared},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("YNAB_BUDGETID", "budget")
			t.Setenv("YNAB_TOKEN", "token")
			t.Setenv("YNAB_ACCOUNTMAP", `{"bank":"ynab"}`)
			if test.clearedEnv == "" {
				t.Setenv("YNAB_CLEARED", "")
				if err := os.Unsetenv("YNAB_CLEARED"); err != nil {
					t.Fatal(err)
				}
			} else {
				t.Setenv("YNAB_CLEARED", test.clearedEnv)
			}

			writer, err := NewWriterFromEnv()
			if err != nil {
				t.Fatal(err)
			}
			if writer.Config.BudgetID != "budget" || writer.Config.Cleared != test.want {
				t.Fatalf("NewWriterFromEnv() config = %#v", writer.Config)
			}
		})
	}
}

func testHTTPWriter(client httpClient, baseURL string) (Writer, ynabber.Transaction) {
	now := time.Now().UTC()
	source := ynabber.Transaction{
		Account: ynabber.Account{IBAN: "test-iban"},
		ID:      "transaction-id",
		Date:    time.Date(now.Year(), now.Month(), now.Day()-1, 12, 0, 0, 0, time.UTC),
		Amount:  1000,
	}
	return Writer{
		Config: Config{
			BudgetID:   "budget-id",
			Token:      "token",
			AccountMap: AccountMap{"test-iban": "account-id"},
			Cleared:    Cleared,
		},
		logger:  slog.Default(),
		client:  client,
		baseURL: baseURL,
	}, source
}

type errorHTTPClient struct {
	err error
}

func (c errorHTTPClient) Do(*http.Request) (*http.Response, error) {
	return nil, c.err
}

type recordingHTTPClient struct {
	request  *http.Request
	response *http.Response
}

func (c *recordingHTTPClient) Do(request *http.Request) (*http.Response, error) {
	c.request = request
	return c.response, nil
}
