package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	if !strings.Contains(err.Error(), "bad import") {
		t.Fatalf("expected Actual import error, got %v", err)
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
	if !strings.Contains(err.Error(), "Account not found") {
		t.Fatalf("expected middleware error message, got %v", err)
	}
}

func TestImportTransactionsSanitizesUnexpectedErrorBody(t *testing.T) {
	const body = "private-response-body"
	header := make(http.Header)
	header.Set("Content-Type", "text/html; charset=utf-8")
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     header,
		}, nil
	})
	c := NewClient("https://actual.example.com", "key", "pass", &http.Client{Transport: transport}, nil)

	_, err := c.ImportTransactions(context.Background(), "budget-1", "account-1", []Transaction{}, ImportTransactionsOptions{})
	if err == nil {
		t.Fatal("expected middleware error")
	}
	if strings.Contains(err.Error(), body) {
		t.Fatalf("error exposes unexpected response body: %v", err)
	}
	// The shape of the body is what lets an operator tell a proxy's HTML error
	// page from a malformed JSON response, so it has to survive sanitizing.
	for _, want := range []string{"unexpected response", fmt.Sprintf("%d byte", len(body)), "text/html"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %v, want it to contain %q", err, want)
		}
	}
}

// TestImportTransactionsRequestSizeIsAdditive pins the JSON shape the writer's
// batch planner depends on: a request is a fixed envelope around an array, so
// its encoded size is the empty envelope, plus each element's own size, plus
// one separator byte between consecutive elements. The planner relies on that
// identity to stay linear. If the encoding ever gains indentation or another
// envelope field, this fails here rather than silently letting the planner
// emit oversized requests.
func TestImportTransactionsRequestSizeIsAdditive(t *testing.T) {
	transactions := []Transaction{
		{Account: "account-1", Date: "2024-05-10", Amount: 100, PayeeName: `quoted "payee" <>&`, ImportedID: "id-1"},
		{Account: "account-1", Date: "2024-05-11", Amount: -250, Notes: `path\with\slashes`, ImportedID: "id-2"},
		{Account: "account-1", Date: "2024-05-12", Amount: 0, ImportedPayee: strings.Repeat("ø", 50)},
	}
	opts := ImportTransactionsOptions{DefaultCleared: true, ReimportDeleted: true, DryRun: true}

	envelope, err := ImportTransactionsRequestSize([]Transaction{}, opts)
	if err != nil {
		t.Fatalf("measuring envelope: %v", err)
	}

	// The planner measures the envelope with a non-nil empty slice on purpose: a
	// nil slice encodes as null rather than [], which would shift every
	// element's computed size.
	nilEnvelope, err := ImportTransactionsRequestSize(nil, opts)
	if err != nil {
		t.Fatalf("measuring nil envelope: %v", err)
	}
	if nilEnvelope == envelope {
		t.Fatal("expected a nil slice to encode differently from an empty array")
	}

	for n := 1; n <= len(transactions); n++ {
		want := envelope + (n - 1) // one separator byte between elements
		for i := range n {
			single, err := ImportTransactionsRequestSize(transactions[i:i+1], opts)
			if err != nil {
				t.Fatalf("measuring transaction %d: %v", i+1, err)
			}
			want += single - envelope
		}

		got, err := ImportTransactionsRequestSize(transactions[:n], opts)
		if err != nil {
			t.Fatalf("measuring %d transactions: %v", n, err)
		}
		if got != want {
			t.Fatalf("size of %d transactions = %d, want %d from the additive model", n, got, want)
		}
	}
}

func TestDescribeBody(t *testing.T) {
	tests := []struct {
		name        string
		payload     string
		contentType string
		want        string
	}{
		{name: "empty", payload: "", contentType: "text/html", want: "empty body"},
		{name: "media type parameters stripped", payload: "abc", contentType: "application/json; charset=utf-8", want: "3 byte application/json body"},
		{name: "missing content type", payload: "abc", contentType: "", want: "3 byte unknown content type body"},
		{name: "oversized content type capped", payload: "ab", contentType: strings.Repeat("x", maxMediaTypeLen+10), want: fmt.Sprintf("2 byte %s... body", strings.Repeat("x", maxMediaTypeLen))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := describeBody([]byte(tt.payload), tt.contentType); got != tt.want {
				t.Fatalf("describeBody() = %q, want %q", got, tt.want)
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

func TestImportTransactionsDoesNotLogPayloads(t *testing.T) {
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
	for _, sensitive := range []string{"private-payee", "private-note"} {
		if strings.Contains(got, sensitive) {
			t.Fatalf("trace log contains sensitive value %q: %s", sensitive, got)
		}
	}
	for _, diagnostic := range []string{"transactions=1", "request_bytes=", "response_bytes="} {
		if !strings.Contains(got, diagnostic) {
			t.Fatalf("trace log is missing %q: %s", diagnostic, got)
		}
	}
}
