package actual

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
	"github.com/martinohansen/ynabber/writer/actual/client"
)

type compatibilityRequest struct {
	Method             string          `json:"method"`
	Path               string          `json:"path"`
	ContentType        string          `json:"content_type"`
	APIKey             string          `json:"api_key"`
	EncryptionPassword string          `json:"encryption_password"`
	Body               json.RawMessage `json:"body"`
}

type compatibilityTransport struct {
	request *http.Request
	body    []byte
}

func (c *compatibilityTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(request.Body)
	_ = request.Body.Close()
	if err != nil {
		return nil, err
	}
	c.request = request
	c.body = body
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"added":["one","two"],"updated":[]}}`)),
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
			accountID:     "actual-enablebanking-account",
		},
		{
			name:          "nordigen",
			canonicalPath: "../../reader/nordigen/testdata/canonical.json",
			goldenPath:    "testdata/nordigen.request.golden",
			accountKey:    "XX000000000000000000",
			accountID:     "actual-nordigen-account",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			transactions := loadCompatibilityTransactions(t, test.canonicalPath)
			transport := &compatibilityTransport{}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			writer := Writer{
				Config: Config{
					BudgetID:        "fixture-budget",
					AccountMap:      AccountMap{test.accountKey: test.accountID},
					Cleared:         true,
					ReimportDeleted: true,
					DryRun:          true,
				},
				logger: logger,
				now: func() time.Time {
					return time.Date(2026, time.August, 19, 12, 0, 0, 0, time.UTC)
				},
				client: client.NewClient(
					"https://actual.example.test",
					"fixture-api-key",
					"fixture-encryption-password",
					&http.Client{Transport: transport},
					logger,
				),
			}

			if err := writer.Bulk(context.Background(), transactions); err != nil {
				t.Fatalf("write canonical transactions: %v", err)
			}
			if transport.request == nil {
				t.Fatal("Actual client did not send a request")
			}

			got := compatibilityRequest{
				Method:             transport.request.Method,
				Path:               transport.request.URL.EscapedPath(),
				ContentType:        transport.request.Header.Get("Content-Type"),
				APIKey:             transport.request.Header.Get("x-api-key"),
				EncryptionPassword: transport.request.Header.Get("budget-encryption-password"),
				Body:               transport.body,
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
