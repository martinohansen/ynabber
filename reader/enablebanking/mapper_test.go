package enablebanking

import (
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

// TestMapper tests that the Mapper method produces a valid transaction
func TestMapper(t *testing.T) {
	reader := Reader{
		logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
		Config: Config{ASPSP: "DNB"},
	}

	account := AccountInfo{
		UID:         "acc-123",
		IBAN:        randomTestIBAN(t),
		DisplayName: "Test",
	}

	tx := EBTransaction{
		TransactionID:        "tx-123",
		BookingDate:          "2024-01-15",
		CreditDebitIndicator: "CRDT",
		TransactionAmount: struct {
			Currency string `json:"currency"`
			Amount   string `json:"amount"`
		}{Amount: "100.00"},
	}

	// Mapper should produce a valid transaction regardless of ASPSP
	result, err := reader.Mapper(account, tx)
	if err != nil {
		t.Fatalf("Mapper failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected transaction, got nil")
	}
}

// TestParseAmount tests amount parsing
func TestParseAmount(t *testing.T) {
	tests := []struct {
		name      string
		amountStr string
		expected  float64
		wantErr   bool
	}{
		{"positive whole", "1000.00", 1000.00, false},
		{"positive decimal", "123.45", 123.45, false},
		{"zero", "0.00", 0.00, false},
		{"small amount", "0.01", 0.01, false},
		{"large amount", "999999.99", 999999.99, false},
		{"invalid", "abc", 0, true},
		{"empty", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseAmount(tt.amountStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseAmount error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && result != tt.expected {
				t.Errorf("parseAmount = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestParseDateFlexible tests date parsing
func TestParseDateFlexible(t *testing.T) {
	tests := []struct {
		name    string
		dateStr string
		wantErr bool
	}{
		{"valid date", "2024-01-15", false},
		{"valid datetime", "2024-01-15T13:45:00", false},
		{"valid rfc3339", "2024-01-15T13:45:00Z", false},
		{"leap year", "2024-02-29", false},
		{"year end", "2024-12-31", false},
		{"invalid format", "15-01-2024", true},
		{"invalid month", "2024-13-01", true},
		{"invalid day", "2024-02-30", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseDateFlexible(tt.dateStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDateFlexible error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				expected := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
				if tt.dateStr == "2024-01-15" && result != expected {
					t.Errorf("parseDateFlexible = %v, expected %v", result, expected)
				}
			}
		})
	}
}

// TestDefaultMapperCredit tests default mapper with credit transaction
func TestDefaultMapperCredit(t *testing.T) {
	reader := Reader{
		logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	account := AccountInfo{
		UID:         "acc-123",
		IBAN:        randomTestIBAN(t),
		DisplayName: "My Account",
	}

	tx := EBTransaction{
		TransactionID:        "tx-456",
		BookingDate:          "2024-01-15",
		CreditDebitIndicator: "CRDT",
		TransactionAmount: struct {
			Currency string `json:"currency"`
			Amount   string `json:"amount"`
		}{
			Currency: "NOK",
			Amount:   "1500.00",
		},
		RemittanceInformation: []string{"Salary"},
	}

	result, err := reader.defaultMapper(account, tx)
	if err != nil {
		t.Fatalf("defaultMapper failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected transaction, got nil")
	}

	// Amount should be positive for credit
	if int64(result.Amount) != 1500000 {
		t.Errorf("expected 1500000 milliunits, got %d", int64(result.Amount))
	}

	if result.Payee != "Salary" {
		t.Errorf("expected payee 'Salary', got '%s'", result.Payee)
	}
}

// TestDefaultMapperDebit tests default mapper with debit transaction
func TestDefaultMapperDebit(t *testing.T) {
	reader := Reader{
		logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	account := AccountInfo{
		UID:  "acc-123",
		IBAN: randomTestIBAN(t),
	}

	tx := EBTransaction{
		TransactionID:        "tx-789",
		BookingDate:          "2024-01-15",
		CreditDebitIndicator: "DBIT",
		TransactionAmount: struct {
			Currency string `json:"currency"`
			Amount   string `json:"amount"`
		}{
			Amount: "500.00",
		},
	}

	result, err := reader.defaultMapper(account, tx)
	if err != nil {
		t.Fatalf("defaultMapper failed: %v", err)
	}

	// Amount should be negative for debit
	if int64(result.Amount) != -500000 {
		t.Errorf("expected -500000 milliunits, got %d", int64(result.Amount))
	}
}

// TestExtractPayeeRemittance tests payee extraction from remittance
func TestExtractPayeeRemittance(t *testing.T) {
	reader := Reader{
		logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	tx := EBTransaction{
		RemittanceInformation: []string{"My Payee", "Additional info"},
		TransactionID:         "tx-123",
	}

	payee := reader.extractPayee(tx)
	if payee != "My Payee" {
		t.Errorf("expected 'My Payee', got '%s'", payee)
	}
}

// TestExtractPayeeDebtor tests payee extraction from debtor
func TestExtractPayeeDebtor(t *testing.T) {
	reader := Reader{
		logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	tx := EBTransaction{
		Debtor: map[string]interface{}{
			"name": "Debtor Name",
		},
		TransactionID: "tx-123",
	}

	payee := reader.extractPayee(tx)
	if payee != "Debtor Name" {
		t.Errorf("expected 'Debtor Name', got '%s'", payee)
	}
}

// TestExtractPayeeCreditor tests payee extraction from creditor
func TestExtractPayeeCreditor(t *testing.T) {
	reader := Reader{
		logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	tx := EBTransaction{
		Creditor: map[string]interface{}{
			"name": "Creditor Name",
		},
		TransactionID: "tx-123",
	}

	payee := reader.extractPayee(tx)
	if payee != "Creditor Name" {
		t.Errorf("expected 'Creditor Name', got '%s'", payee)
	}
}

// TestExtractPayeeFallback tests payee extraction fallback to transaction ID
func TestExtractPayeeFallback(t *testing.T) {
	reader := Reader{
		logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	tx := EBTransaction{
		TransactionID: "tx-fallback",
	}

	payee := reader.extractPayee(tx)
	if payee != "tx-fallback" {
		t.Errorf("expected 'tx-fallback', got '%s'", payee)
	}
}

// TestExtractMemoMultipleRemittance tests that when multiple remittance strings
// are present, [1:] are joined as the memo (the first element becomes the Payee).
func TestExtractMemoMultipleRemittance(t *testing.T) {
	reader := Reader{
		logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	tx := EBTransaction{
		RemittanceInformation: []string{"Payee", "Memo Info", "Extra"},
	}

	memo := reader.extractMemo(tx)
	if memo != "Memo Info Extra" {
		t.Errorf("expected 'Memo Info Extra', got '%s'", memo)
	}
}

// TestExtractMemoSingleRemittance tests that a single remittance string is used
// as the memo fallback (matching Nordigen behaviour: Payee gets the stripped
// value, Memo keeps the full raw text for YNAB context).
func TestExtractMemoSingleRemittance(t *testing.T) {
	reader := Reader{
		Config: Config{PayeeStrip: []string{" VISA 12345"}},
		logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	tx := EBTransaction{
		RemittanceInformation: []string{"Rema 1000 Slemmestad VISA 12345"},
		TransactionID:         "txn-1",
	}

	memo := reader.extractMemo(tx)
	if memo != "Rema 1000 Slemmestad VISA 12345" {
		t.Errorf("expected full raw remittance string, got '%s'", memo)
	}
	// Sanity-check that Payee does get the stripped version via defaultMapper
	payee := reader.extractPayee(tx)
	payee = strip(payee, reader.Config.PayeeStrip)
	if payee != "Rema 1000 Slemmestad" {
		t.Errorf("expected stripped payee 'Rema 1000 Slemmestad', got '%s'", payee)
	}
}

// TestExtractMemoNote tests memo extraction from note field
func TestExtractMemoNote(t *testing.T) {
	reader := Reader{
		logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	tx := EBTransaction{
		Note: "Transaction Note",
	}

	memo := reader.extractMemo(tx)
	if memo != "Transaction Note" {
		t.Errorf("expected 'Transaction Note', got '%s'", memo)
	}
}

// TestExtractMemoEmpty tests memo extraction when empty
func TestExtractMemoEmpty(t *testing.T) {
	reader := Reader{
		logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	tx := EBTransaction{}

	memo := reader.extractMemo(tx)
	if memo != "" {
		t.Errorf("expected empty memo, got '%s'", memo)
	}
}

// TestMapperWithDifferentASPSPs tests that Mapper produces a valid transaction
// for any ASPSP value (there is no bank-specific dispatch).
func TestMapperWithDifferentASPSPs(t *testing.T) {
	tests := []struct {
		name  string
		aspsp string
	}{
		{"DNB", "DNB"},
		{"Nordea", "Nordea"},
		{"SparBank", "SparBank"},
		{"Unknown", "UnknownBank"},
	}

	account := AccountInfo{
		UID:  "acc-123",
		IBAN: randomTestIBAN(t),
	}

	tx := EBTransaction{
		TransactionID:        "tx-123",
		BookingDate:          "2024-01-15",
		CreditDebitIndicator: "CRDT",
		TransactionAmount: struct {
			Currency string `json:"currency"`
			Amount   string `json:"amount"`
		}{Amount: "100.00"},
		RemittanceInformation: []string{"Test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := Reader{
				logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
				Config: Config{ASPSP: tt.aspsp},
			}

			result, err := reader.Mapper(account, tx)
			if err != nil {
				t.Fatalf("Mapper failed: %v", err)
			}

			if result == nil {
				t.Fatal("expected transaction, got nil")
			}
		})
	}
}

// TestPayeeStrip tests that PayeeStrip configuration removes specified words
func TestPayeeStrip(t *testing.T) {
	tests := []struct {
		name          string
		payeeStrip    []string
		remittance    []string
		expectedPayee string
	}{
		{
			name:          "strip single word",
			payeeStrip:    []string{"Visa "},
			remittance:    []string{"Visa Google Play"},
			expectedPayee: "Google Play",
		},
		{
			name:          "strip multiple words",
			payeeStrip:    []string{"Varekjøp,", "Visa ", "Lån,"},
			remittance:    []string{"Varekjøp, Rema Slemmestad"},
			expectedPayee: "Rema Slemmestad",
		},
		{
			name:          "strip with trailing separator",
			payeeStrip:    []string{"Overføring "},
			remittance:    []string{"Overføring 123 Oslo Bank"},
			expectedPayee: "123 Oslo Bank",
		},
		{
			name:          "no strip applied",
			payeeStrip:    []string{"foo,", "bar"},
			remittance:    []string{"Google Play"},
			expectedPayee: "Google Play",
		},
		{
			name:          "strip all",
			payeeStrip:    []string{"Giro,"},
			remittance:    []string{"Giro,"},
			expectedPayee: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := Reader{
				logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
				Config: Config{PayeeStrip: tt.payeeStrip},
			}

			account := AccountInfo{
				UID:  "acc-123",
				IBAN: randomTestIBAN(t),
			}

			tx := EBTransaction{
				TransactionID:        "tx-123",
				BookingDate:          "2024-01-15",
				CreditDebitIndicator: "CRDT",
				TransactionAmount: struct {
					Currency string `json:"currency"`
					Amount   string `json:"amount"`
				}{Amount: "100.00"},
				RemittanceInformation: tt.remittance,
			}

			result, err := reader.Mapper(account, tx)
			if err != nil {
				t.Fatalf("Mapper failed: %v", err)
			}

			if result.Payee != tt.expectedPayee {
				t.Errorf("expected payee '%s', got '%s'", tt.expectedPayee, result.Payee)
			}
		})
	}
}

// TestPayeeTruncation tests that payees exceeding 200 characters are truncated
func TestPayeeTruncation(t *testing.T) {
	tests := []struct {
		name          string
		remittance    []string
		expectedLen   int
		maxLen        int
	}{
		{
			name:        "under limit",
			remittance:  []string{"Short Payee"},
			expectedLen: 11,
			maxLen:      200,
		},
		{
			name:        "at limit",
			remittance:  []string{strings.Repeat("A", 200)},
			expectedLen: 200,
			maxLen:      200,
		},
		{
			name:        "exceeds limit",
			remittance:  []string{strings.Repeat("A", 250)},
			expectedLen: 200,
			maxLen:      200,
		},
		{
			name:        "way over limit",
			remittance:  []string{strings.Repeat("A", 500)},
			expectedLen: 200,
			maxLen:      200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := Reader{
				logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
			}

			account := AccountInfo{
				UID:  "acc-123",
				IBAN: randomTestIBAN(t),
			}

			tx := EBTransaction{
				TransactionID:        "tx-123",
				BookingDate:          "2024-01-15",
				CreditDebitIndicator: "CRDT",
				TransactionAmount: struct {
					Currency string `json:"currency"`
					Amount   string `json:"amount"`
				}{Amount: "100.00"},
				RemittanceInformation: tt.remittance,
			}

			result, err := reader.Mapper(account, tx)
			if err != nil {
				t.Fatalf("Mapper failed: %v", err)
			}

			if len(result.Payee) > tt.maxLen {
				t.Errorf("payee exceeds max length: got %d, max %d", len(result.Payee), tt.maxLen)
			}

			if len(result.Payee) != tt.expectedLen {
				t.Errorf("expected payee length %d, got %d", tt.expectedLen, len(result.Payee))
			}
		})
	}
}

// TestPayeeStripAndTruncate tests that both stripping and truncation work together
func TestPayeeStripAndTruncate(t *testing.T) {
	reader := Reader{
		logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
		Config: Config{
			PayeeStrip: []string{"Visa "},
		},
	}

	account := AccountInfo{
		UID:  "acc-123",
		IBAN: randomTestIBAN(t),
	}

	// Payee is "Visa " + 200 characters = 205 total
	// After strip: 200 characters
	// After truncate: 200 characters (at limit)
	longPayee := "Visa " + strings.Repeat("A", 200)
	tx := EBTransaction{
		TransactionID:        "tx-123",
		BookingDate:          "2024-01-15",
		CreditDebitIndicator: "CRDT",
		TransactionAmount: struct {
			Currency string `json:"currency"`
			Amount   string `json:"amount"`
		}{Amount: "100.00"},
		RemittanceInformation: []string{longPayee},
	}

	result, err := reader.Mapper(account, tx)
	if err != nil {
		t.Fatalf("Mapper failed: %v", err)
	}

	// After stripping "Visa ", we have 200 A's, which fits exactly
	if len(result.Payee) != 200 {
		t.Errorf("expected payee length 200, got %d", len(result.Payee))
	}

	if result.Payee != strings.Repeat("A", 200) {
		t.Errorf("expected 200 A's, got '%s'", result.Payee)
	}
}
