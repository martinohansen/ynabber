package wealthreader

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/martinohansen/ynabber"
)

func TestParseResponseUnauthorized(t *testing.T) {
	body := []byte(`{"success":false,"error":{"code":2020,"message":"token invalid"}}`)
	_, err := parseResponse(body)
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("error = %v, want ErrUnauthorized", err)
	}
}

func TestParseResponseOtherError(t *testing.T) {
	body := []byte(`{"success":false,"error":{"code":5000,"message":"maintenance"}}`)
	_, err := parseResponse(body)
	if err == nil || errors.Is(err, ErrUnauthorized) {
		t.Fatalf("error = %v, want a non-auth error", err)
	}
}

func TestClientFetch(t *testing.T) {
	fixture, err := os.ReadFile("testdata/transactions.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/entities/" {
			t.Errorf("path = %s, want /entities/", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if r.Form.Get("api_key") != "11dfdeab" {
			t.Errorf("api_key = %s", r.Form.Get("api_key"))
		}
		if r.Form.Get("code") != "bbva" {
			t.Errorf("code = %s", r.Form.Get("code"))
		}
		if r.Form.Get("token") != "FRJ0mHlaqZwLzu" {
			t.Errorf("token = %s", r.Form.Get("token"))
		}
		if r.Form.Get("date_from") != "2024-01-01" {
			t.Errorf("date_from = %s", r.Form.Get("date_from"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer server.Close()

	client := NewClient(Config{
		ApiKey:       "11dfdeab",
		ApiBase:      server.URL,
		ProductTypes: "accounts",
		FromDate:     mustDate(t, "2024-01-01"),
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	got, err := client.Fetch(context.Background(), Session{Token: "FRJ0mHlaqZwLzu", Code: "bbva"})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if !got.Success || len(got.Payload.Accounts) != 1 {
		t.Fatalf("unexpected response: %+v", got)
	}
}

func TestClientFetchRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("slow down"))
	}))
	defer server.Close()

	client := NewClient(Config{ApiBase: server.URL}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, err := client.Fetch(context.Background(), Session{Token: "t", Code: "bbva"})
	if !errors.Is(err, ErrRateLimit) {
		t.Fatalf("error = %v, want ErrRateLimit", err)
	}
}

func TestBulkWithSessionFile(t *testing.T) {
	fixture, err := os.ReadFile("testdata/transactions.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var wantCanonical []ynabber.Transaction
	canonical, err := os.ReadFile("testdata/canonical.json")
	if err != nil {
		t.Fatalf("read canonical: %v", err)
	}
	if err := json.Unmarshal(canonical, &wantCanonical); err != nil {
		t.Fatalf("decode canonical: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer server.Close()

	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")
	if err := os.WriteFile(sessionPath, []byte(`{"token":"FRJ0mHlaqZwLzu","code":"bbva"}`), 0o600); err != nil {
		t.Fatalf("write session: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := Config{
		ApiKey:       "11dfdeab",
		Code:         "bbva",
		RedirectURL:  "https://example.com/ok",
		SessionFile:  sessionPath,
		FromDate:     mustDate(t, "2024-01-01"),
		ProductTypes: "accounts",
		ApiBase:      server.URL,
		OAuthBase:    server.URL,
	}
	reader := Reader{
		Config: cfg,
		Auth:   NewAuth(cfg, logger),
		Client: NewClient(cfg, logger),
		logger: logger,
	}

	got, err := reader.Bulk(context.Background())
	if err != nil {
		t.Fatalf("Bulk() error = %v", err)
	}
	if diff := cmp.Diff(wantCanonical, got); diff != "" {
		t.Errorf("Bulk() mismatch (-want +got):\n%s", diff)
	}
}

func TestRunnerOneShotMode(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	want := []ynabber.Transaction{{
		ID:     "tx-test",
		Payee:  "Test",
		Amount: 10000,
	}}
	calls := 0
	reader := Reader{
		Config: Config{Interval: 0},
		logger: logger,
		bulkFn: func(context.Context) ([]ynabber.Transaction, error) {
			calls++
			return want, nil
		},
	}
	out := make(chan []ynabber.Transaction, 1)

	if err := reader.Runner(context.Background(), out); err != nil {
		t.Fatalf("Runner() error = %v", err)
	}
	if calls != 1 {
		t.Errorf("bulk calls = %d, want 1", calls)
	}
	got := <-out
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("batch mismatch (-want +got):\n%s", diff)
	}
}

func TestString(t *testing.T) {
	reader := Reader{}
	if reader.String() != "wealthreader" {
		t.Errorf("String() = %s", reader.String())
	}
}

func TestMaskIdentifier(t *testing.T) {
	if got := maskIdentifier("ES4914651234561234567890"); !strings.Contains(got, "...") {
		t.Errorf("maskIdentifier() = %s", got)
	}
	if got := maskIdentifier("short"); got != "****" {
		t.Errorf("maskIdentifier(short) = %s", got)
	}
}
