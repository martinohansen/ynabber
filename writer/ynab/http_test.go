package ynab

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

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusTooManyRequests)
		_, _ = response.Write([]byte(`{"error":{"id":"429","name":"too_many_requests"}}`))
	}))
	t.Cleanup(server.Close)

	writer, source := testHTTPWriter(server.Client(), server.URL)
	err := writer.Bulk(context.Background(), []ynabber.Transaction{source})
	if err == nil {
		t.Fatal("Bulk() error = nil, want API error")
	}
	if got, want := err.Error(), "failed to send request: 429 Too Many Requests"; got != want {
		t.Errorf("Bulk() error = %q, want %q", got, want)
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
