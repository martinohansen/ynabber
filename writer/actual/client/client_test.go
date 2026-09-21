package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/martinohansen/ynabber/internal/log"
)

type capturingTransport struct {
	requests []*http.Request
	bodies   [][]byte
}

func (c *capturingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	req.Body.Close()
	req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	c.requests = append(c.requests, req)
	c.bodies = append(c.bodies, bodyBytes)
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"data":{"added":["id-1"],"updated":[],"errors":[]}}`)),
		Header:     make(http.Header),
	}, nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestImportTransactions(t *testing.T) {
	transport := &capturingTransport{}
	c := NewClient("https://actual.example.com", "key", "pass", &http.Client{Transport: transport}, nil)

	cleared := true
	tx := []Transaction{{
		Account:    "account-1",
		Date:       "2024-05-10",
		Amount:     1234,
		PayeeName:  "Payee",
		ImportedID: "id-1",
		Cleared:    &cleared,
	}}

	result, err := c.ImportTransactions(context.Background(), "budget-1", "account-1", tx, ImportTransactionsOptions{DefaultCleared: false, ReimportDeleted: true})
	if err != nil {
		t.Fatalf("ImportTransactions() error = %v", err)
	}
	if result.Added != 1 || result.Updated != 0 {
		t.Fatalf("unexpected result %+v", result)
	}

	if len(transport.requests) != 1 {
		t.Fatalf("expected one request, got %d", len(transport.requests))
	}

	req := transport.requests[0]
	if req.Method != http.MethodPost {
		t.Fatalf("expected POST got %s", req.Method)
	}
	if req.Header.Get("x-api-key") != "key" {
		t.Fatalf("expected api key header")
	}
	if req.Header.Get("budget-encryption-password") != "pass" {
		t.Fatalf("expected encryption header")
	}

	var body importTransactionsRequest
	if err := json.Unmarshal(transport.bodies[0], &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if len(body.Transactions) != 1 {
		t.Fatalf("unexpected transaction count %d", len(body.Transactions))
	}
	if body.DefaultCleared {
		t.Fatalf("expected defaultCleared false")
	}
	if !body.ReimportDeleted {
		t.Fatalf("expected reimportDeleted true")
	}

	requestBytes, err := ImportTransactionsRequestSize(tx, ImportTransactionsOptions{DefaultCleared: false, ReimportDeleted: true})
	if err != nil {
		t.Fatalf("ImportTransactionsRequestSize() error = %v", err)
	}
	if requestBytes != len(transport.bodies[0]) {
		t.Fatalf("measured request size = %d, sent body = %d", requestBytes, len(transport.bodies[0]))
	}
}

func TestImportTransactionsDryRun(t *testing.T) {
	transport := &capturingTransport{}
	c := NewClient("https://actual.example.com", "", "", &http.Client{Transport: transport}, nil)

	_, err := c.ImportTransactions(context.Background(), "budget-1", "account-1", []Transaction{}, ImportTransactionsOptions{DryRun: true})
	if err != nil {
		t.Fatalf("ImportTransactions() error = %v", err)
	}

	var body importTransactionsRequest
	if err := json.Unmarshal(transport.bodies[0], &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if !body.DryRun {
		t.Fatalf("expected dryRun true")
	}
}

func TestImportTransactionsReturnsImportErrors(t *testing.T) {
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"data":{"added":["added-id"],"updated":["updated-id"],"errors":[{"message":"bad import"}]}}`)),
			Header:     make(http.Header),
		}, nil
	})
	c := NewClient("https://actual.example.com", "key", "pass", &http.Client{Transport: transport}, nil)

	result, err := c.ImportTransactions(context.Background(), "budget-1", "account-1", []Transaction{{
		Account: "account-1",
		Date:    "2024-05-10",
		Amount:  1234,
	}}, ImportTransactionsOptions{})
	if err == nil {
		t.Fatalf("expected import error")
	}
	if got, want := err.Error(), "actual import errors: 1"; got != want {
		t.Fatalf("import error = %q, want %q", got, want)
	}
	if result.Added != 1 || result.Updated != 1 {
		t.Fatalf("result = %+v, want reported partial counts", result)
	}
}

func TestImportTransactionsRejectsIncompleteSuccessResponse(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "missing data", body: `{}`, want: "missing data object"},
		{name: "null data", body: `{"data":null}`, want: "missing data object"},
		{name: "missing errors", body: `{"data":{"added":[],"updated":[]}}`, want: "missing data.errors array"},
		{name: "null errors", body: `{"data":{"added":[],"updated":[],"errors":null}}`, want: "missing data.errors array"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(tt.body)),
					Header:     make(http.Header),
				}, nil
			})
			c := NewClient("https://actual.example.com", "key", "pass", &http.Client{Transport: transport}, nil)

			_, err := c.ImportTransactions(context.Background(), "budget-1", "account-1", nil, ImportTransactionsOptions{})
			if err == nil {
				t.Fatal("expected response contract error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want it to contain %q", err, tt.want)
			}
		})
	}
}

// TestImportTransactionsToleratesMissingCounts covers the other half of the
// response contract: added and updated only feed metrics, so an import that
// otherwise succeeded must not fail because the response omitted a counter.
func TestImportTransactionsToleratesMissingCounts(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantAdded   int
		wantUpdated int
	}{
		{name: "missing added", body: `{"data":{"updated":["a"],"errors":[]}}`, wantAdded: 0, wantUpdated: 1},
		{name: "null added", body: `{"data":{"added":null,"updated":["a"],"errors":[]}}`, wantAdded: 0, wantUpdated: 1},
		{name: "missing updated", body: `{"data":{"added":["a","b"],"errors":[]}}`, wantAdded: 2, wantUpdated: 0},
		{name: "both absent", body: `{"data":{"errors":[]}}`, wantAdded: 0, wantUpdated: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(tt.body)),
					Header:     make(http.Header),
				}, nil
			})
			var logs strings.Builder
			logger := slog.New(slog.NewTextHandler(&logs, nil))
			c := NewClient("https://actual.example.com", "key", "pass", &http.Client{Transport: transport}, logger)

			result, err := c.ImportTransactions(context.Background(), "budget-1", "account-1", nil, ImportTransactionsOptions{})
			if err != nil {
				t.Fatalf("ImportTransactions() error = %v, want success", err)
			}
			if result.Added != tt.wantAdded || result.Updated != tt.wantUpdated {
				t.Fatalf("result = %+v, want added=%d updated=%d", result, tt.wantAdded, tt.wantUpdated)
			}
			if !strings.Contains(logs.String(), "omitted import counts") {
				t.Fatalf("expected a warning about omitted counts, got: %s", logs.String())
			}
		})
	}
}

func TestImportTransactionsReturnsMiddlewareError(t *testing.T) {
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader(`{"error":"Account not found"}`)),
			Header:     make(http.Header),
		}, nil
	})
	c := NewClient("https://actual.example.com", "key", "pass", &http.Client{Transport: transport}, nil)

	_, err := c.ImportTransactions(context.Background(), "budget-1", "account-1", []Transaction{{
		Account: "account-1",
		Date:    "2024-05-10",
		Amount:  1234,
	}}, ImportTransactionsOptions{})
	if err == nil {
		t.Fatalf("expected middleware error")
	}
	if !strings.Contains(err.Error(), "actual import request returned status 404") {
		t.Fatalf("expected middleware status, got %v", err)
	}
	if strings.Contains(err.Error(), "Account not found") {
		t.Fatalf("middleware error includes response text: %v", err)
	}
}

func TestImportErrorsKeepResponseDataAtTrace(t *testing.T) {
	for _, test := range []struct {
		name       string
		status     int
		body, want string
	}{
		{"HTML", 500, "<html>private-key private-password</html>", "actual import request returned status 500"},
		{"JSON error", 400, `{"error":"private-key private-password"}`, "actual import request returned status 400"},
		{"import errors", 200, `{"data":{"added":["added-id"],"updated":[],"errors":[{"message":"private-key"},"private-password"]}}`, "actual import errors: 2"},
	} {
		t.Run(test.name, func(t *testing.T) {
			transport := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: test.status, Body: io.NopCloser(strings.NewReader(test.body)), Header: make(http.Header)}, nil
			})
			var logs bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: log.LevelTrace}))
			c := NewClient("https://actual.example.com", "private-key", "private-password", &http.Client{Transport: transport}, logger)
			_, err := c.ImportTransactions(context.Background(), "budget", "account", nil, ImportTransactionsOptions{})
			if err == nil || err.Error() != test.want {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
			for _, value := range []string{"private-key", "private-password"} {
				if !strings.Contains(logs.String(), value) {
					t.Errorf("trace omits %q: %s", value, logs.String())
				}
			}
		})
	}
}

func TestImportTransactionsEscapesPathComponents(t *testing.T) {
	transport := &capturingTransport{}
	c := NewClient("https://actual.example.com", "key", "", &http.Client{Transport: transport}, nil)

	_, err := c.ImportTransactions(context.Background(), "budget/1", "account?2", []Transaction{}, ImportTransactionsOptions{})
	if err != nil {
		t.Fatalf("ImportTransactions() error = %v", err)
	}

	if len(transport.requests) != 1 {
		t.Fatalf("expected one request, got %d", len(transport.requests))
	}
	got := transport.requests[0].URL.EscapedPath()
	want := "/v1/budgets/budget%2F1/accounts/account%3F2/transactions/import"
	if got != want {
		t.Fatalf("expected URL path %q, got %q", want, got)
	}
}

func TestImportTransactionsLogsPayloadsAtTrace(t *testing.T) {
	transport := &capturingTransport{}
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: log.LevelTrace}))
	c := NewClient("https://actual.example.com", "key", "pass", &http.Client{Transport: transport}, logger)

	_, err := c.ImportTransactions(context.Background(), "budget-1", "account-1", []Transaction{{
		Account:    "account-1",
		Date:       "2024-05-10",
		Amount:     1234,
		PayeeName:  "private-payee",
		Notes:      "private-note",
		ImportedID: "id-1",
	}}, ImportTransactionsOptions{})
	if err != nil {
		t.Fatalf("ImportTransactions() error = %v", err)
	}

	got := logs.String()
	for _, private := range []string{"private-payee", "private-note"} {
		if !strings.Contains(got, private) {
			t.Fatalf("trace log is missing private value %q: %s", private, got)
		}
	}
	for _, diagnostic := range []string{"transactions=1", "request_bytes=", "response_bytes="} {
		if !strings.Contains(got, diagnostic) {
			t.Fatalf("trace log is missing %q: %s", diagnostic, got)
		}
	}
}
