package ynab

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/martinohansen/ynabber"
)

type compatibilityRequest struct {
	Method        string          `json:"method"`
	Path          string          `json:"path"`
	ContentType   string          `json:"content_type"`
	Authorization string          `json:"authorization"`
	Body          json.RawMessage `json:"body"`
}

type compatibilityClient struct {
	request *http.Request
	body    []byte
}

func (c *compatibilityClient) Do(request *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(request.Body)
	_ = request.Body.Close()
	if err != nil {
		return nil, err
	}
	c.request = request
	c.body = body
	return &http.Response{
		StatusCode: http.StatusCreated,
		Status:     "201 Created",
		Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"transaction_ids":["one","two"]}}`)),
		Header:     make(http.Header),
	}, nil
}

func TestCompatibilityMatrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		canonicalPath string
		goldenPath    string
		accountKey    string
		accountID     string
	}{
		{
			name:          "enable banking",
			canonicalPath: "../../reader/enablebanking/testdata/canonical.json",
			goldenPath:    "testdata/enablebanking.request.golden",
			accountKey:    "fixture-session-account",
			accountID:     "ynab-enablebanking-account",
		},
		{
			name:          "nordigen",
			canonicalPath: "../../reader/nordigen/testdata/canonical.json",
			goldenPath:    "testdata/nordigen.request.golden",
			accountKey:    "XX000000000000000000",
			accountID:     "ynab-nordigen-account",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			transactions := loadCompatibilityTransactions(t, test.canonicalPath)
			client := &compatibilityClient{}
			writer := Writer{
				Config: Config{
					BudgetID:   "fixture-budget",
					Token:      "fixture-token",
					AccountMap: AccountMap{test.accountKey: test.accountID},
					Cleared:    Reconciled,
				},
				logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
				client:  client,
				baseURL: "https://ynab.example.test/v1",
				now: func() time.Time {
					return time.Date(2026, time.August, 19, 12, 0, 0, 0, time.UTC)
				},
			}

			if err := writer.Bulk(context.Background(), transactions); err != nil {
				t.Fatalf("write canonical transactions: %v", err)
			}
			if client.request == nil {
				t.Fatal("YNAB client did not send a request")
			}

			got := compatibilityRequest{
				Method:        client.request.Method,
				Path:          client.request.URL.EscapedPath(),
				ContentType:   client.request.Header.Get("Content-Type"),
				Authorization: client.request.Header.Get("Authorization"),
				Body:          client.body,
			}
			compareCompatibilityGolden(t, test.goldenPath, got)
		})
	}
}

func loadCompatibilityTransactions(t *testing.T, path string) []ynabber.Transaction {
	t.Helper()

	fixture, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read canonical fixture: %v", err)
	}
	var transactions []ynabber.Transaction
	if err := json.Unmarshal(fixture, &transactions); err != nil {
		t.Fatalf("decode canonical fixture: %v", err)
	}
	return transactions
}

func compareCompatibilityGolden(t *testing.T, path string, got compatibilityRequest) {
	t.Helper()

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read request golden: %v", err)
	}
	encoded, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatalf("encode request snapshot: %v", err)
	}
	wantLines := strings.Split(string(bytes.TrimSpace(want)), "\n")
	gotLines := strings.Split(string(encoded), "\n")
	if diff := cmp.Diff(wantLines, gotLines); diff != "" {
		t.Errorf("request golden mismatch (-want +got):\n%s", diff)
	}
}
