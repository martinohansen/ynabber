package enablebanking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/martinohansen/ynabber"
)

// TestReaderRetryHandler tests the retry handler for error handling
func TestReaderRetryHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	reader := Reader{
		logger: logger,
	}

	tests := []struct {
		name     string
		inputErr error
		wantErr  bool
	}{
		{
			name:     "regular error",
			inputErr: errors.New("some error"),
			wantErr:  true,
		},
		{
			name:     "nil error",
			inputErr: nil,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr := reader.retryHandler(context.Background(), tt.inputErr)
			if (gotErr != nil) != tt.wantErr {
				t.Errorf("retryHandler() error = %v, want error %v", gotErr, tt.wantErr)
			}
			if gotErr != nil && tt.inputErr != nil && gotErr.Error() != tt.inputErr.Error() {
				t.Errorf("retryHandler() error = %v, want %v", gotErr, tt.inputErr)
			}
		})
	}
}

// TestRunnerOneShotMode verifies that the production runner fetches and sends
// one batch when the interval is zero.
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
		t.Fatalf("Bulk calls = %d, want 1", calls)
	}

	select {
	case got := <-out:
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("batch mismatch (-want +got):\n%s", diff)
		}
	default:
		t.Fatal("Runner() did not send a batch")
	}
}

// TestRunnerContinuousMode drives the production runner with a controlled
// timer, so repeated execution and cancellation need no wall-clock sleeps.
func TestRunnerContinuousMode(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	interval := 5 * time.Minute
	ticks := make(chan time.Time)
	waits := make(chan time.Duration, 2)
	calls := 0
	reader := Reader{
		Config: Config{Interval: interval},
		logger: logger,
		bulkFn: func(context.Context) ([]ynabber.Transaction, error) {
			calls++
			return []ynabber.Transaction{{ID: ynabber.ID(fmt.Sprintf("tx-%d", calls))}}, nil
		},
		afterFn: func(delay time.Duration) <-chan time.Time {
			waits <- delay
			return ticks
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	out := make(chan []ynabber.Transaction)
	runnerErr := make(chan error, 1)
	go func() {
		runnerErr <- reader.Runner(ctx, out)
	}()

	for wantCall := 1; wantCall <= 2; wantCall++ {
		select {
		case batch := <-out:
			if len(batch) != 1 || batch[0].ID != ynabber.ID(fmt.Sprintf("tx-%d", wantCall)) {
				t.Fatalf("batch %d = %+v", wantCall, batch)
			}
		case <-time.After(time.Second):
			t.Fatalf("Runner() did not send batch %d", wantCall)
		}

		select {
		case got := <-waits:
			if got != interval {
				t.Fatalf("wait after batch %d = %v, want %v", wantCall, got, interval)
			}
		case <-time.After(time.Second):
			t.Fatalf("Runner() did not wait after batch %d", wantCall)
		}

		if wantCall == 1 {
			ticks <- time.Now()
		}
	}

	cancel()
	select {
	case err := <-runnerErr:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Runner() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Runner() did not stop after cancellation")
	}
	if calls != 2 {
		t.Fatalf("Bulk calls = %d, want 2", calls)
	}
}

// TestRunnerCancellationWhileSending verifies that cancellation releases a
// runner whose output channel has no receiver.
func TestRunnerCancellationWhileSending(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	fetched := make(chan struct{})
	reader := Reader{
		Config: Config{Interval: 0},
		logger: logger,
		bulkFn: func(context.Context) ([]ynabber.Transaction, error) {
			close(fetched)
			return []ynabber.Transaction{{ID: "tx-test"}}, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	runnerErr := make(chan error, 1)
	go func() {
		runnerErr <- reader.Runner(ctx, make(chan []ynabber.Transaction))
	}()

	select {
	case <-fetched:
	case <-time.After(time.Second):
		t.Fatal("Runner() did not fetch a batch")
	}
	cancel()

	select {
	case err := <-runnerErr:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Runner() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Runner() remained blocked on output")
	}
}

// TestRunnerErrorPropagation verifies that the production runner returns
// one-shot read failures unchanged.
func TestRunnerErrorPropagation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	wantErr := errors.New("test error")
	reader := Reader{
		Config: Config{Interval: 0},
		logger: logger,
		bulkFn: func(context.Context) ([]ynabber.Transaction, error) {
			return nil, wantErr
		},
	}

	err := reader.Runner(context.Background(), make(chan []ynabber.Transaction))
	if !errors.Is(err, wantErr) {
		t.Fatalf("Runner() error = %v, want %v", err, wantErr)
	}
}

// TestRunnerRetriesHandledError verifies that the production runner retries a
// handled failure, sends the recovered batch, and resumes its normal interval.
func TestRunnerRetriesHandledError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	interval := time.Hour
	retryDelay := 30 * time.Second
	ticks := make(chan time.Time)
	waits := make(chan time.Duration, 2)
	calls := 0
	reader := Reader{
		Config:     Config{Interval: interval},
		logger:     logger,
		retryDelay: retryDelay,
		bulkFn: func(context.Context) ([]ynabber.Transaction, error) {
			calls++
			if calls == 1 {
				return nil, ErrRateLimit
			}
			return []ynabber.Transaction{{ID: "recovered"}}, nil
		},
		afterFn: func(delay time.Duration) <-chan time.Time {
			waits <- delay
			return ticks
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	runnerErr := make(chan error, 1)
	out := make(chan []ynabber.Transaction)
	go func() {
		runnerErr <- reader.Runner(ctx, out)
	}()

	select {
	case got := <-waits:
		if got != retryDelay {
			t.Fatalf("retry wait = %v, want %v", got, retryDelay)
		}
	case <-time.After(time.Second):
		t.Fatal("Runner() did not wait before retrying")
	}
	ticks <- time.Now()

	select {
	case batch := <-out:
		if len(batch) != 1 || batch[0].ID != "recovered" {
			t.Fatalf("recovered batch = %+v", batch)
		}
	case <-time.After(time.Second):
		t.Fatal("Runner() did not send the recovered batch")
	}

	select {
	case got := <-waits:
		if got != interval {
			t.Fatalf("normal wait = %v, want %v", got, interval)
		}
	case <-time.After(time.Second):
		t.Fatal("Runner() did not resume its normal interval")
	}

	cancel()
	select {
	case err := <-runnerErr:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Runner() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Runner() did not stop after cancellation")
	}
	if calls != 2 {
		t.Fatalf("Bulk calls = %d, want 2", calls)
	}
}

func TestBulkReauthorizesServerRejectedSession(t *testing.T) {
	const (
		rejectedAccountUID    = "rejected-account"
		replacementAccountUID = "replacement-account"
		replacementIBAN       = "NO9812345678901"
	)

	tempDir := t.TempDir()
	sessionPath := filepath.Join(tempDir, "session.json")
	keyPath := filepath.Join(tempDir, "key.pem")

	sessionData, err := json.Marshal(Session{
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
		ValidUntil: time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339),
		Accounts:   []AccountInfo{{UID: rejectedAccountUID}},
	})
	if err != nil {
		t.Fatalf("marshaling session: %v", err)
	}
	if err := os.WriteFile(sessionPath, sessionData, 0600); err != nil {
		t.Fatalf("writing session: %v", err)
	}
	if err := os.WriteFile(keyPath, generateTestKeyPair(t), 0600); err != nil {
		t.Fatalf("writing PEM key: %v", err)
	}

	redirectReader, redirectWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating redirect input pipe: %v", err)
	}
	defer redirectReader.Close()
	defer redirectWriter.Close()

	rejectedTransactionRequests := 0
	replacementTransactionRequests := 0
	authorizationRequests := 0
	sessionRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/accounts/"+rejectedAccountUID+"/transactions"):
			rejectedTransactionRequests++
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"Session expired","code":401,"error":"EXPIRED_SESSION"}`))
		case strings.Contains(r.URL.Path, "/accounts/"+replacementAccountUID+"/transactions"):
			replacementTransactionRequests++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"transactions":[{
					"entry_reference":"replacement-entry",
					"transaction_id":"replacement-transaction",
					"booking_date":"2024-01-15",
					"credit_debit_indicator":"CRDT",
					"status":"BOOK",
					"transaction_amount":{"currency":"NOK","amount":"100.00"},
					"remittance_information":["Replacement transaction"]
				}],
				"pending":[]
			}`))
		case r.URL.Path == "/aspsps":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"aspsps":[]}`))
		case r.URL.Path == "/auth":
			authorizationRequests++
			if authorizationRequests > 1 {
				http.Error(w, "unexpected repeated authorization", http.StatusConflict)
				return
			}
			var request AuthorizationRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decoding authorization request: %v", err)
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			if _, err := fmt.Fprintf(redirectWriter,
				"https://callback.example/?code=replacement-code&state=%s\n", request.State); err != nil {
				t.Errorf("writing redirect input: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"url":"https://bank.example/authorize","id":"authorization-1"}`))
		case r.URL.Path == "/sessions":
			sessionRequests++
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{
				"createdAt":"%s",
				"accounts":[{"uid":%q,"account_id":{"iban":%q}}],
				"access":{"valid_until":"%s"}
			}`,
				time.Now().UTC().Format(time.RFC3339),
				replacementAccountUID,
				replacementIBAN,
				time.Now().UTC().Add(24*time.Hour).Format(time.RFC3339),
			)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	authConfig := Config{
		AppID:       "test-app",
		ASPSP:       "test-bank",
		Country:     "NO",
		PEMFile:     keyPath,
		SessionFile: sessionPath,
	}
	reader := Reader{
		Config: Config{
			FromDate: Date(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
			ToDate:   Date(time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)),
		},
		Auth: Auth{
			Config:        authConfig,
			baseURL:       server.URL,
			httpClient:    server.Client(),
			redirectInput: redirectReader,
			logger:        logger,
		},
		Client: &Client{
			BaseURL:    server.URL,
			HTTPClient: server.Client(),
			logger:     logger,
		},
		logger: logger,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	batch, err := reader.Bulk(ctx)
	if err != nil {
		t.Fatalf("Bulk() returned error after successful reauthorization: %v", err)
	}
	if len(batch) != 1 {
		t.Fatalf("output batch transactions = %d, want 1", len(batch))
	}
	if batch[0].ID != "replacement-entry" {
		t.Errorf("output transaction ID = %q, want %q", batch[0].ID, "replacement-entry")
	}
	if rejectedTransactionRequests != 1 {
		t.Errorf("rejected-session transaction requests = %d, want 1", rejectedTransactionRequests)
	}
	if replacementTransactionRequests != 1 {
		t.Errorf("replacement-session transaction requests = %d, want 1", replacementTransactionRequests)
	}
	if authorizationRequests != 1 {
		t.Errorf("authorization requests = %d, want 1", authorizationRequests)
	}
	if sessionRequests != 1 {
		t.Errorf("session requests = %d, want 1", sessionRequests)
	}

	persistedData, err := os.ReadFile(sessionPath)
	if err != nil {
		t.Fatalf("reading replacement session: %v", err)
	}
	var persisted Session
	if err := json.Unmarshal(persistedData, &persisted); err != nil {
		t.Fatalf("parsing replacement session: %v", err)
	}
	if len(persisted.Accounts) != 1 || persisted.Accounts[0].UID != replacementAccountUID {
		t.Errorf("persisted accounts = %+v, want replacement account %q", persisted.Accounts, replacementAccountUID)
	}
}

func TestBulkStopsWhenFreshSessionIsRejected(t *testing.T) {
	const accountUID = "rejected-account"

	tempDir := t.TempDir()
	sessionPath := filepath.Join(tempDir, "session.json")
	keyPath := filepath.Join(tempDir, "key.pem")

	sessionData, err := json.Marshal(Session{
		CreatedAt:  time.Now().UTC().AddDate(0, 0, -11).Format(time.RFC3339),
		ValidUntil: time.Now().UTC().Add(-time.Second).Format(time.RFC3339),
		Accounts:   []AccountInfo{{UID: accountUID}},
	})
	if err != nil {
		t.Fatalf("marshaling session: %v", err)
	}
	if err := os.WriteFile(sessionPath, sessionData, 0600); err != nil {
		t.Fatalf("writing session: %v", err)
	}
	if err := os.WriteFile(keyPath, generateTestKeyPair(t), 0600); err != nil {
		t.Fatalf("writing PEM key: %v", err)
	}

	redirectReader, redirectWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating redirect input pipe: %v", err)
	}
	defer redirectReader.Close()
	defer redirectWriter.Close()

	transactionRequests := 0
	authorizationRequests := 0
	sessionRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/transactions"):
			transactionRequests++
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"Session expired","code":401,"error":"EXPIRED_SESSION"}`))
		case r.URL.Path == "/aspsps":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"aspsps":[]}`))
		case r.URL.Path == "/auth":
			authorizationRequests++
			if authorizationRequests > 1 {
				http.Error(w, "unexpected repeated authorization", http.StatusConflict)
				return
			}
			var request AuthorizationRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decoding authorization request: %v", err)
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			if _, err := fmt.Fprintf(redirectWriter,
				"https://callback.example/?code=replacement-code&state=%s\n", request.State); err != nil {
				t.Errorf("writing redirect input: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"url":"https://bank.example/authorize","id":"authorization-1"}`))
		case r.URL.Path == "/sessions":
			sessionRequests++
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{
				"createdAt":"%s",
				"accounts":[{"uid":%q}],
				"access":{"valid_until":"%s"}
			}`,
				time.Now().UTC().Format(time.RFC3339),
				accountUID,
				time.Now().UTC().Add(24*time.Hour).Format(time.RFC3339),
			)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	authConfig := Config{
		AppID:       "test-app",
		ASPSP:       "test-bank",
		Country:     "NO",
		PEMFile:     keyPath,
		SessionFile: sessionPath,
	}
	reader := Reader{
		Config: Config{
			FromDate: Date(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
			ToDate:   Date(time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)),
		},
		Auth: Auth{
			Config:        authConfig,
			baseURL:       server.URL,
			httpClient:    server.Client(),
			redirectInput: redirectReader,
			logger:        logger,
		},
		Client: &Client{
			BaseURL:    server.URL,
			HTTPClient: server.Client(),
			logger:     logger,
		},
		logger: logger,
	}

	_, err = reader.Bulk(context.Background())
	if !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("Bulk() error = %v, want ErrSessionExpired after fresh session rejection", err)
	}
	if transactionRequests != 1 {
		t.Errorf("transaction requests = %d, want 1", transactionRequests)
	}
	if authorizationRequests != 1 {
		t.Errorf("authorization requests = %d, want 1", authorizationRequests)
	}
	if sessionRequests != 1 {
		t.Errorf("session requests = %d, want 1", sessionRequests)
	}
	if _, statErr := os.Stat(sessionPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("rejected fresh session file still exists; os.Stat error = %v", statErr)
	}
}

func TestRejectedFreshSessionIsFatalWhenInvalidationFails(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "session")
	if err := os.Mkdir(sessionPath, 0700); err != nil {
		t.Fatalf("creating session directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sessionPath, "keep"), []byte("test"), 0600); err != nil {
		t.Fatalf("making session directory non-empty: %v", err)
	}

	reader := Reader{Auth: Auth{Config: Config{SessionFile: sessionPath}}}
	err := reader.rejectedNewSessionError(ErrUnauthorized)
	if !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("rejectedNewSessionError() error = %v, want ErrSessionExpired", err)
	}
	if !strings.Contains(err.Error(), "discarding rejected session") {
		t.Fatalf("rejectedNewSessionError() error = %v, want invalidation context", err)
	}
}

// TestRetryHandlerContinuousMode tests retry/backoff behaviour when running in
// continuous mode (Interval > 0).
func TestRetryHandlerContinuousMode(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tiny := 1 * time.Millisecond // keep tests fast

	tests := []struct {
		name    string
		err     error
		wantNil bool // true if retryHandler should return nil (retry signalled)
	}{
		{
			name:    "rate limit is retried",
			err:     ErrRateLimit,
			wantNil: true,
		},
		{
			name:    "transient error is retried",
			err:     errors.New("connection refused"),
			wantNil: true,
		},
		{
			name:    "ErrSessionExpired is fatal even in continuous mode",
			err:     ErrSessionExpired,
			wantNil: false,
		},
		{
			name:    "raw ErrUnauthorized is fatal",
			err:     ErrUnauthorized,
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := Reader{
				Config:     Config{Interval: 1 * time.Hour},
				logger:     logger,
				retryDelay: tiny,
			}
			got := reader.retryHandler(context.Background(), tt.err)
			if tt.wantNil && got != nil {
				t.Errorf("expected nil (retry signalled), got %v", got)
			}
			if !tt.wantNil && got == nil {
				t.Errorf("expected non-nil error (fatal), got nil")
			}
		})
	}
}

// TestRetryHandlerContextCancellation verifies that a context cancellation
// during the backoff wait is surfaced immediately for both transient errors
// and rate-limit errors.
func TestRetryHandlerContextCancellation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tests := []struct {
		name string
		err  error
	}{
		{
			name: "transient error — cancelled context returns ctx.Err()",
			err:  errors.New("some transient error"),
		},
		{
			name: "rate limit error — cancelled context returns ctx.Err()",
			err:  ErrRateLimit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := Reader{
				Config:     Config{Interval: 1 * time.Hour},
				logger:     logger,
				retryDelay: 10 * time.Second, // long enough that ctx fires first
			}

			ctx, cancel := context.WithCancel(context.Background())
			cancel() // cancel immediately

			got := reader.retryHandler(ctx, tt.err)
			if !errors.Is(got, context.Canceled) {
				t.Errorf("expected context.Canceled, got %v", got)
			}
		})
	}
}

// TestNextDailyRetryTime verifies that the retry target is always the
// following calendar day at rateLimitRetryHour:rateLimitRetryMinute.
func TestNextDailyRetryTime(t *testing.T) {
	loc := time.UTC

	tests := []struct {
		name    string
		now     time.Time
		wantDay int // expected Day() of result
		wantH   int
		wantM   int
	}{
		{
			name:    "early morning — still tomorrow",
			now:     time.Date(2024, 1, 10, 3, 0, 0, 0, loc),
			wantDay: 11,
			wantH:   rateLimitRetryHour,
			wantM:   rateLimitRetryMinute,
		},
		{
			name:    "exactly at retry time — still tomorrow",
			now:     time.Date(2024, 1, 10, rateLimitRetryHour, rateLimitRetryMinute, 0, 0, loc),
			wantDay: 11,
			wantH:   rateLimitRetryHour,
			wantM:   rateLimitRetryMinute,
		},
		{
			name:    "late night — still tomorrow",
			now:     time.Date(2024, 1, 10, 23, 59, 59, 0, loc),
			wantDay: 11,
			wantH:   rateLimitRetryHour,
			wantM:   rateLimitRetryMinute,
		},
		{
			name:    "month boundary",
			now:     time.Date(2024, 1, 31, 12, 0, 0, 0, loc),
			wantDay: 1, // Feb 1
			wantH:   rateLimitRetryHour,
			wantM:   rateLimitRetryMinute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nextDailyRetryTime(tt.now)
			if got.Day() != tt.wantDay {
				t.Errorf("Day() = %d, want %d", got.Day(), tt.wantDay)
			}
			if got.Hour() != tt.wantH {
				t.Errorf("Hour() = %d, want %d", got.Hour(), tt.wantH)
			}
			if got.Minute() != tt.wantM {
				t.Errorf("Minute() = %d, want %d", got.Minute(), tt.wantM)
			}
			if got.Second() != 0 || got.Nanosecond() != 0 {
				t.Errorf("expected zero seconds/nanoseconds, got %v", got)
			}
			if !got.After(tt.now) {
				t.Errorf("retry time %v is not after now %v", got, tt.now)
			}
		})
	}
}
