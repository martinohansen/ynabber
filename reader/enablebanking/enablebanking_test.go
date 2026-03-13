package enablebanking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/martinohansen/ynabber"
)

// TestClientGetAccountTransactions tests fetching transactions from API
func TestClientGetAccountTransactions(t *testing.T) {
	// Create mock server
	mockResponse := `{
		"transactions": [
			{
				"transaction_id": "tx-123",
				"booking_date": "2024-01-15",
				"credit_debit_indicator": "CRDT",
				"transaction_amount": {
					"currency": "NOK",
					"amount": "1000.00"
				},
				"remittance_information": ["Salary payment"]
			}
		],
		"pending": []
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}

		if r.Header.Get("Authorization") == "" {
			t.Error("missing Authorization header")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, mockResponse)
	}))
	defer server.Close()

	client := &Client{
		BaseURL:    server.URL,
		HTTPClient: http.DefaultClient,
		logger:     slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	txResp, err := client.GetAccountTransactions(context.Background(), "test-token", "account-123", "2024-01-01", "2024-01-31")
	if err != nil {
		t.Fatalf("GetAccountTransactions failed: %v", err)
	}

	if len(txResp.Transactions) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(txResp.Transactions))
	}

	if txResp.Transactions[0].TransactionID != "tx-123" {
		t.Errorf("expected transaction ID 'tx-123', got '%s'", txResp.Transactions[0].TransactionID)
	}
}

// TestClientGetAccountTransactionsQueryParams verifies that GetAccountTransactions
// sends the correct date_from / date_to query parameter names required by the
// EnableBanking API. Using the wrong names (e.g. "from"/"to") causes the API to
// silently ignore the filter and return all transactions.
func TestClientGetAccountTransactionsQueryParams(t *testing.T) {
	tests := []struct {
		name     string
		fromDate string
		toDate   string
	}{
		{
			name:     "standard date range",
			fromDate: "2024-01-01",
			toDate:   "2024-01-31",
		},
		{
			name:     "single day range",
			fromDate: "2024-06-15",
			toDate:   "2024-06-15",
		},
		{
			name:     "cross-year range",
			fromDate: "2023-12-01",
			toDate:   "2024-01-01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedURL string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedURL = r.URL.String()
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"transactions":[],"pending":[]}`)
			}))
			defer server.Close()

			client := &Client{
				BaseURL:    server.URL,
				HTTPClient: http.DefaultClient,
				logger:     slog.New(slog.NewTextHandler(os.Stderr, nil)),
			}

			_, err := client.GetAccountTransactions(context.Background(), "token", "acct-1", tt.fromDate, tt.toDate)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			q := func(u, param, want string) {
				t.Helper()
				parsed, _ := url.Parse(u)
				got := parsed.Query().Get(param)
				if got != want {
					t.Errorf("query param %q = %q, want %q (full URL: %s)", param, got, want, u)
				}
			}

			q(capturedURL, "date_from", tt.fromDate)
			q(capturedURL, "date_to", tt.toDate)

			// Ensure the old wrong parameter names are NOT present
			parsed, _ := url.Parse(capturedURL)
			if parsed.Query().Has("from") {
				t.Errorf("unexpected legacy param 'from' present in URL: %s", capturedURL)
			}
			if parsed.Query().Has("to") {
				t.Errorf("unexpected legacy param 'to' present in URL: %s", capturedURL)
			}
		})
	}
}

func TestClientGetAccountTransactionsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error": "invalid token"}`)
	}))
	defer server.Close()

	client := &Client{
		BaseURL:    server.URL,
		HTTPClient: http.DefaultClient,
		logger:     slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	_, err := client.GetAccountTransactions(context.Background(), "bad-token", "account-123", "2024-01-01", "2024-01-31")
	if err == nil {
		t.Fatal("expected error for API failure")
	}
}

// TestReaderMapTransaction tests transaction mapping
func TestReaderMapTransaction(t *testing.T) {
	reader := Reader{
		logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}
	iban := randomTestIBAN(t)

	account := AccountInfo{
		UID:         "acc-123",
		AccountID:   AccountID{IBAN: iban},
		DisplayName: "Test Account",
	}

	ebTx := EBTransaction{
		EntryReference:       "202401150000456", // stable bank-assigned reference
		TransactionID:        "tx-456",          // session-scoped; must not be used as ID
		BookingDate:          "2024-01-15",
		CreditDebitIndicator: "CRDT",
		TransactionAmount: struct {
			Currency string `json:"currency"`
			Amount   string `json:"amount"`
		}{
			Currency: "NOK",
			Amount:   "500.50",
		},
		RemittanceInformation: []string{"Payment for services"},
		Status:                "BOOK",
	}

	tx, err := reader.Mapper(account, ebTx)
	if err != nil {
		t.Fatalf("Mapper failed: %v", err)
	}

	if tx == nil {
		t.Fatal("expected transaction, got nil")
	}

	if tx.ID != "202401150000456" {
		t.Errorf("expected ID '202401150000456', got '%s'", tx.ID)
	}

	if tx.Account.IBAN != iban {
		t.Errorf("expected IBAN '%s', got '%s'", iban, tx.Account.IBAN)
	}

	if tx.Payee != "Payment for services" {
		t.Errorf("expected payee 'Payment for services', got '%s'", tx.Payee)
	}

	// Credit should be positive, amount should be 500,500 milliunits
	expectedAmount := int64(500500)
	if int64(tx.Amount) != expectedAmount {
		t.Errorf("expected amount %d, got %d", expectedAmount, int64(tx.Amount))
	}
}

// TestReaderMapTransactionDebit tests transaction mapping with debit
func TestReaderMapTransactionDebit(t *testing.T) {
	reader := Reader{
		logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}
	iban := randomTestIBAN(t)

	account := AccountInfo{
		UID:       "acc-123",
		AccountID: AccountID{IBAN: iban},
	}

	ebTx := EBTransaction{
		TransactionID:        "tx-789",
		BookingDate:          "2024-01-15",
		CreditDebitIndicator: "DBIT",
		TransactionAmount: struct {
			Currency string `json:"currency"`
			Amount   string `json:"amount"`
		}{
			Currency: "NOK",
			Amount:   "100.00",
		},
	}

	tx, err := reader.Mapper(account, ebTx)
	if err != nil {
		t.Fatalf("Mapper failed: %v", err)
	}

	// Debit should be negative
	if int64(tx.Amount) >= 0 {
		t.Errorf("debit transaction should be negative, got %d", int64(tx.Amount))
	}
}

// TestReaderMapTransactionMissingFields tests error handling for missing fields
func TestReaderMapTransactionMissingFields(t *testing.T) {
	reader := Reader{
		logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	account := AccountInfo{UID: "acc-123"}

	tests := []struct {
		name string
		tx   EBTransaction
	}{
		{
			name: "missing transaction ID",
			tx: EBTransaction{
				BookingDate: "2024-01-15",
			},
		},
		{
			name: "missing booking date",
			tx: EBTransaction{
				TransactionID: "tx-123",
			},
		},
		{
			name: "invalid booking date",
			tx: EBTransaction{
				TransactionID: "tx-123",
				BookingDate:   "invalid-date",
			},
		},
		{
			name: "invalid amount",
			tx: EBTransaction{
				TransactionID: "tx-123",
				BookingDate:   "2024-01-15",
				TransactionAmount: struct {
					Currency string `json:"currency"`
					Amount   string `json:"amount"`
				}{
					Amount: "invalid-amount",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := reader.Mapper(account, tt.tx)
			if err == nil {
				t.Error("expected error for missing/invalid fields")
			}
		})
	}
}

// TestReaderExtractPayee tests payee extraction
func TestReaderExtractPayee(t *testing.T) {
	reader := Reader{
		logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	tests := []struct {
		name     string
		tx       EBTransaction
		expected string
	}{
		{
			name: "from remittance information",
			tx: EBTransaction{
				RemittanceInformation: []string{"Test Payee"},
				TransactionID:         "tx-123",
			},
			expected: "Test Payee",
		},
		{
			name: "fallback to transaction ID",
			tx: EBTransaction{
				TransactionID: "tx-456",
			},
			expected: "tx-456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payee := reader.extractPayee(tt.tx)
			if payee != tt.expected {
				t.Errorf("expected payee '%s', got '%s'", tt.expected, payee)
			}
		})
	}
}

// TestReaderExtractMemo tests memo extraction
func TestReaderExtractMemo(t *testing.T) {
	reader := Reader{
		logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	tests := []struct {
		name     string
		tx       EBTransaction
		expected string
	}{
		{
			name: "single remittance entry used as memo fallback",
			tx: EBTransaction{
				RemittanceInformation: []string{"Rema 1000 Oslo"},
			},
			expected: "Rema 1000 Oslo",
		},
		{
			name: "multiple remittance: joins [1:] as memo",
			tx: EBTransaction{
				RemittanceInformation: []string{"Payee", "Description"},
			},
			expected: "Description",
		},
		{
			name: "three remittance strings: joins [1:] with space",
			tx: EBTransaction{
				RemittanceInformation: []string{"Payee", "Ref 123", "Invoice 456"},
			},
			expected: "Ref 123 Invoice 456",
		},
		{
			name: "from note field",
			tx: EBTransaction{
				Note: "Transaction note",
			},
			expected: "Transaction note",
		},
		{
			name:     "empty memo",
			tx:       EBTransaction{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memo := reader.extractMemo(tt.tx)
			if memo != tt.expected {
				t.Errorf("expected memo '%s', got '%s'", tt.expected, memo)
			}
		})
	}
}

// TestReaderString tests String method
func TestReaderString(t *testing.T) {
	reader := Reader{
		logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	if reader.String() != "enablebanking" {
		t.Errorf("expected 'enablebanking', got '%s'", reader.String())
	}
}

// TestTransactionStructure tests EBTransaction structure
func TestTransactionStructure(t *testing.T) {
	tx := EBTransaction{
		TransactionID: "tx-123",
		BookingDate:   "2024-01-15",
		Status:        "BOOK",
		TransactionAmount: struct {
			Currency string `json:"currency"`
			Amount   string `json:"amount"`
		}{
			Currency: "NOK",
			Amount:   "100.00",
		},
	}

	if tx.TransactionID != "tx-123" {
		t.Errorf("unexpected transaction ID: %s", tx.TransactionID)
	}

	if tx.TransactionAmount.Amount != "100.00" {
		t.Errorf("unexpected amount: %s", tx.TransactionAmount.Amount)
	}
}

// TestClientNewClient tests client creation
func TestClientNewClient(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	client := NewClient(logger)

	if client == nil {
		t.Fatal("expected client, got nil")
	}

	if client.BaseURL != enableBankingAPIBase {
		t.Errorf("expected base URL '%s', got '%s'", enableBankingAPIBase, client.BaseURL)
	}

	if client.HTTPClient == nil {
		t.Fatal("expected HTTP client, got nil")
	}
}

// TestTransactionsResponseUnmarshal tests JSON unmarshaling
func TestTransactionsResponseUnmarshal(t *testing.T) {
	jsonData := []byte(`{
		"transactions": [
			{
				"transaction_id": "tx-1",
				"booking_date": "2024-01-15",
				"credit_debit_indicator": "CRDT",
				"transaction_amount": {
					"currency": "NOK",
					"amount": "1000.00"
				}
			}
		],
		"pending": [
			{
				"transaction_id": "tx-2",
				"booking_date": "2024-01-16",
				"credit_debit_indicator": "DBIT",
				"transaction_amount": {
					"currency": "NOK",
					"amount": "500.00"
				}
			}
		]
	}`)

	var resp TransactionsResponse
	err := json.Unmarshal(jsonData, &resp)
	if err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if len(resp.Transactions) != 1 {
		t.Errorf("expected 1 transaction, got %d", len(resp.Transactions))
	}

	if len(resp.Pending) != 1 {
		t.Errorf("expected 1 pending, got %d", len(resp.Pending))
	}
}

// TestAccountInfoStructure tests AccountInfo structure
func TestAccountInfoStructure(t *testing.T) {
	iban := randomTestIBAN(t)
	account := AccountInfo{
		UID: "uid-123",
		AccountID: AccountID{
			IBAN: iban,
			Other: AccountIDOther{
				Identification: "86011117947",
				SchemeName:     "BBAN",
			},
		},
		Currency:    "NOK",
		Name:        "Checking",
		DisplayName: "My Checking Account",
		OwnerName:   "John Doe",
		AccountType: "CACC",
		Status:      "enabled",
	}

	if account.UID != "uid-123" {
		t.Errorf("unexpected UID: %s", account.UID)
	}

	if account.AccountID.IBAN != iban {
		t.Errorf("unexpected IBAN: %s", account.AccountID.IBAN)
	}

	if account.Currency != "NOK" {
		t.Errorf("unexpected currency: %s", account.Currency)
	}
}

// TestReaderMapTransactionDate tests date parsing
func TestReaderMapTransactionDate(t *testing.T) {
	reader := Reader{
		logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}
	iban := randomTestIBAN(t)

	account := AccountInfo{
		UID:       "acc-123",
		AccountID: AccountID{IBAN: iban},
	}

	ebTx := EBTransaction{
		TransactionID:        "tx-123",
		BookingDate:          "2024-02-22",
		CreditDebitIndicator: "CRDT",
		TransactionAmount: struct {
			Currency string `json:"currency"`
			Amount   string `json:"amount"`
		}{
			Amount: "100.00",
		},
	}

	tx, err := reader.Mapper(account, ebTx)
	if err != nil {
		t.Fatalf("Mapper failed: %v", err)
	}

	expectedDate := time.Date(2024, 2, 22, 0, 0, 0, 0, time.UTC)
	if tx.Date != expectedDate {
		t.Errorf("expected date %v, got %v", expectedDate, tx.Date)
	}
}

// TestYnabberTransaction tests ynabber transaction structure
func TestYnabberTransaction(t *testing.T) {
	iban := randomTestIBAN(t)
	tx := ynabber.Transaction{
		Account: ynabber.Account{
			ID:   "acc-123",
			Name: "Test Account",
			IBAN: iban,
		},
		ID:     "tx-456",
		Date:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		Payee:  "Test Payee",
		Memo:   "Test memo",
		Amount: ynabber.Milliunits(500500),
	}

	if tx.Account.IBAN != iban {
		t.Errorf("unexpected IBAN: %s", tx.Account.IBAN)
	}

	if tx.Payee != "Test Payee" {
		t.Errorf("unexpected payee: %s", tx.Payee)
	}
}

// ---------------------------------------------------------------------------
// ErrRateLimit — HTTP 429 handling (GetAccountTransactions)
// ---------------------------------------------------------------------------

// TestClientGetAccountTransactionsRateLimit verifies that a 429 response wraps
// ErrRateLimit so callers can use errors.Is for retry decisions.
func TestClientGetAccountTransactionsRateLimit(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
		wantIsRL   bool // errors.Is(err, ErrRateLimit)
	}{
		{
			name:       "HTTP 429 returns ErrRateLimit",
			statusCode: http.StatusTooManyRequests,
			wantErr:    true,
			wantIsRL:   true,
		},
		{
			name:       "HTTP 500 does not return ErrRateLimit",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
			wantIsRL:   false,
		},
		{
			name:       "HTTP 401 returns ErrUnauthorized (not ErrRateLimit)",
			statusCode: http.StatusUnauthorized,
			wantErr:    true,
			wantIsRL:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				fmt.Fprintf(w, `{"error":"status %d"}`, tt.statusCode)
			}))
			defer server.Close()

			client := &Client{
				BaseURL:    server.URL,
				HTTPClient: http.DefaultClient,
				logger:     slog.New(slog.NewTextHandler(os.Stderr, nil)),
			}

			_, err := client.GetAccountTransactions(context.Background(), "token", "acc", "2024-01-01", "2024-01-31")
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantIsRL && !errors.Is(err, ErrRateLimit) {
				t.Errorf("expected errors.Is(err, ErrRateLimit) = true, got false; err = %v", err)
			}
			if !tt.wantIsRL && err != nil && errors.Is(err, ErrRateLimit) {
				t.Errorf("expected errors.Is(err, ErrRateLimit) = false, got true")
			}
			if tt.statusCode == http.StatusUnauthorized && !errors.Is(err, ErrUnauthorized) {
				t.Errorf("HTTP 401: expected errors.Is(err, ErrUnauthorized) = true; err = %v", err)
			}
		})
	}
}

// TestMaskIdentifier verifies that maskIdentifier masks PII correctly.
func TestMaskIdentifier(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"typical IBAN", "NO9812345678901", "NO98...8901"},
		{"masked CPAN", "540111******9999", "5401...9999"},
		{"short — below threshold", "NO981234", "****"},
		{"exactly 8 chars", "ABCD1234", "****"},
		{"9 chars — just above threshold", "ABCD12345", "ABCD...2345"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskIdentifier(tt.input)
			if got != tt.want {
				t.Errorf("maskIdentifier(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestBulkMapsStableIDFromSession verifies the end-to-end behaviour of Bulk()
// after the #152 fix: the IBAN stored in account_id.iban in the session file
// must flow through to the mapped transaction's Account.IBAN.
//
// Any unexpected HTTP call fails the test immediately via the catch-all handler.
func TestBulkMapsStableIDFromSession(t *testing.T) {
	const (
		accountUID  = "test-uid-stable"
		accountIBAN = "NO9812345678901"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if strings.Contains(r.URL.Path, "/transactions") {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{
				"transactions": [{
					"transaction_id": "tx-stable-001",
					"booking_date": "2024-01-15",
					"credit_debit_indicator": "CRDT",
					"status": "BOOK",
					"transaction_amount": {"currency": "NOK", "amount": "100.00"},
					"remittance_information": ["Test payment"]
				}],
				"pending": []
			}`)
			return
		}

		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Session file uses the new account_id nested structure as returned by the
	// EnableBanking /sessions API.
	sessionJSON := fmt.Sprintf(`{
		"createdAt": "2026-01-01T00:00:00Z",
		"valid_until": "2099-01-01T00:00:00Z",
		"accounts": [{
			"uid": %q,
			"account_id": {
				"iban": %q,
				"other": {
					"identification": "12345678901",
					"scheme_name": "BBAN"
				}
			},
			"currency": "NOK",
			"display_name": "Test Account"
		}]
	}`, accountUID, accountIBAN)

	sessionFile, err := os.CreateTemp("", "test-session-*.json")
	if err != nil {
		t.Fatalf("creating temp session file: %v", err)
	}
	defer os.Remove(sessionFile.Name())
	if _, err := sessionFile.WriteString(sessionJSON); err != nil {
		sessionFile.Close()
		t.Fatalf("writing session file: %v", err)
	}
	sessionFile.Close()

	keyData := generateTestKeyPair(t)
	keyFile, err := os.CreateTemp("", "test-key-*.pem")
	if err != nil {
		t.Fatalf("creating temp key file: %v", err)
	}
	defer os.Remove(keyFile.Name())
	if _, err := keyFile.Write(keyData); err != nil {
		keyFile.Close()
		t.Fatalf("writing key file: %v", err)
	}
	keyFile.Close()

	reader := Reader{
		Config: Config{
			FromDate: Date(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
			ToDate:   Date(time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)),
		},
		Auth: Auth{
			Config: Config{
				AppID:       "test-app",
				PEMFile:     keyFile.Name(),
				SessionFile: sessionFile.Name(),
			},
			baseURL:    server.URL,
			httpClient: server.Client(),
			logger:     slog.New(slog.NewTextHandler(os.Stderr, nil)),
		},
		Client: &Client{
			BaseURL:    server.URL,
			HTTPClient: server.Client(),
			logger:     slog.New(slog.NewTextHandler(os.Stderr, nil)),
		},
		logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	txs, err := reader.Bulk(context.Background())
	if err != nil {
		t.Fatalf("Bulk() returned unexpected error: %v", err)
	}

	if len(txs) == 0 {
		t.Fatal("Bulk() returned no transactions; expected at least 1")
	}

	if txs[0].Account.IBAN != accountIBAN {
		t.Errorf("tx.Account.IBAN = %q, want %q\n"+
			"  account_id.iban is not being read from the session;\n"+
			"  StableID() must return the IBAN from account_id.",
			txs[0].Account.IBAN, accountIBAN)
	}
}
