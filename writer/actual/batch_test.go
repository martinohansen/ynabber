package actual

import (
	"fmt"
	"strings"
	"testing"

	"github.com/martinohansen/ynabber/writer/actual/client"
)

func TestBatchTransactionsHonorsExactByteBoundary(t *testing.T) {
	transactions := []client.Transaction{
		{Account: "account-1", Date: "2024-05-10", Amount: 100, PayeeName: "quoted \"payee\"\n<>&", ImportedID: "id-1"},
		{Account: "account-1", Date: "2024-05-11", Amount: 200, Notes: `path\with\slashes`, ImportedID: "id-2"},
	}
	opts := client.ImportTransactionsOptions{DefaultCleared: true}
	twoTransactionBytes, err := client.ImportTransactionsRequestSize(transactions, opts)
	if err != nil {
		t.Fatalf("measuring request: %v", err)
	}

	batches, err := batchTransactions(transactions, opts, twoTransactionBytes, 10)
	if err != nil {
		t.Fatalf("batchTransactions() error = %v", err)
	}
	if len(batches) != 1 || len(batches[0].transactions) != 2 {
		t.Fatalf("expected one batch at the exact boundary, got %+v", batches)
	}
	if batches[0].requestBytes != twoTransactionBytes {
		t.Fatalf("request size = %d, want %d", batches[0].requestBytes, twoTransactionBytes)
	}

	batches, err = batchTransactions(transactions, opts, twoTransactionBytes-1, 10)
	if err != nil {
		t.Fatalf("batchTransactions() error = %v", err)
	}
	if len(batches) != 2 {
		t.Fatalf("expected two batches one byte below the boundary, got %d", len(batches))
	}
}

func TestBatchTransactionsHonorsTransactionLimit(t *testing.T) {
	transactions := make([]client.Transaction, 5)
	for i := range transactions {
		transactions[i] = client.Transaction{ImportedID: fmt.Sprintf("id-%d", i)}
	}

	batches, err := batchTransactions(transactions, client.ImportTransactionsOptions{}, 1_000_000, 2)
	if err != nil {
		t.Fatalf("batchTransactions() error = %v", err)
	}

	wantSizes := []int{2, 2, 1}
	if len(batches) != len(wantSizes) {
		t.Fatalf("got %d batches, want %d", len(batches), len(wantSizes))
	}
	for i, want := range wantSizes {
		if got := len(batches[i].transactions); got != want {
			t.Fatalf("batch %d contains %d transactions, want %d", i+1, got, want)
		}
		if got := cap(batches[i].transactions); got != want {
			t.Fatalf("batch %d capacity = %d, want %d", i+1, got, want)
		}
	}

	offset := 0
	for _, batch := range batches {
		for _, transaction := range batch.transactions {
			if want := transactions[offset].ImportedID; transaction.ImportedID != want {
				t.Fatalf("transaction %d = %q, want %q", offset, transaction.ImportedID, want)
			}
			offset++
		}
	}
}

func TestBatchTransactionsSeparatesDuplicateImportIDs(t *testing.T) {
	transactions := []client.Transaction{
		{ImportedID: "duplicate"},
		{ImportedID: "unique"},
		{ImportedID: "duplicate"},
		{ImportedID: "another"},
	}

	batches, err := batchTransactions(transactions, client.ImportTransactionsOptions{}, 1_000_000, 100)
	if err != nil {
		t.Fatalf("batchTransactions() error = %v", err)
	}
	if len(batches) != 2 {
		t.Fatalf("got %d batches, want 2", len(batches))
	}

	wantIDs := [][]string{{"duplicate", "unique"}, {"duplicate", "another"}}
	for i, batch := range batches {
		if len(batch.transactions) != len(wantIDs[i]) {
			t.Fatalf("batch %d contains %d transactions, want %d", i+1, len(batch.transactions), len(wantIDs[i]))
		}
		for j, transaction := range batch.transactions {
			if transaction.ImportedID != wantIDs[i][j] {
				t.Fatalf("batch %d transaction %d = %q, want %q", i+1, j+1, transaction.ImportedID, wantIDs[i][j])
			}
		}
	}
}

func TestBatchTransactionsLargeUnicodePayload(t *testing.T) {
	// Exactly one count-limited batch would be possible, so splitting this
	// fixture proves that the encoded byte limit is enforced independently.
	const transactionCount = defaultBatchSize
	transactions := make([]client.Transaction, transactionCount)
	for i := range transactions {
		transactions[i] = client.Transaction{
			Account:       "account-1",
			Date:          "2024-05-10",
			Amount:        int64(i * 100),
			PayeeName:     strings.Repeat("ø", 200),
			Notes:         strings.Repeat("æ", 200),
			ImportedPayee: strings.Repeat("å", 200),
			ImportedID:    fmt.Sprintf("id-%03d", i),
		}
	}
	opts := client.ImportTransactionsOptions{ReimportDeleted: true}
	originalBytes, err := client.ImportTransactionsRequestSize(transactions, opts)
	if err != nil {
		t.Fatalf("measuring original request: %v", err)
	}
	if originalBytes <= defaultMaxRequestBytes {
		t.Fatalf("fixture request is only %d bytes; expected more than %d", originalBytes, defaultMaxRequestBytes)
	}

	batches, err := batchTransactions(transactions, opts, defaultMaxRequestBytes, defaultBatchSize)
	if err != nil {
		t.Fatalf("batchTransactions() error = %v", err)
	}
	if len(batches) < 2 {
		t.Fatalf("expected multiple batches, got %d", len(batches))
	}

	offset := 0
	for i, batch := range batches {
		if batch.requestBytes > defaultMaxRequestBytes {
			t.Errorf("batch %d is %d bytes, maximum is %d", i+1, batch.requestBytes, defaultMaxRequestBytes)
		}
		if len(batch.transactions) > defaultBatchSize {
			t.Errorf("batch %d has %d transactions, maximum is %d", i+1, len(batch.transactions), defaultBatchSize)
		}
		measured, err := client.ImportTransactionsRequestSize(batch.transactions, opts)
		if err != nil {
			t.Fatalf("measuring batch %d: %v", i+1, err)
		}
		if measured != batch.requestBytes {
			t.Errorf("batch %d recorded %d bytes, measured %d", i+1, batch.requestBytes, measured)
		}
		for _, transaction := range batch.transactions {
			if want := transactions[offset].ImportedID; transaction.ImportedID != want {
				t.Fatalf("transaction %d = %q, want %q", offset, transaction.ImportedID, want)
			}
			offset++
		}

		if i+1 < len(batches) && len(batch.transactions) < defaultBatchSize {
			candidate := append([]client.Transaction(nil), batch.transactions...)
			candidate = append(candidate, batches[i+1].transactions[0])
			candidateBytes, err := client.ImportTransactionsRequestSize(candidate, opts)
			if err != nil {
				t.Fatalf("measuring expanded batch %d: %v", i+1, err)
			}
			if candidateBytes <= defaultMaxRequestBytes {
				t.Errorf("batch %d is not maximal: one more transaction uses %d bytes", i+1, candidateBytes)
			}
		}
	}

	if offset != transactionCount {
		t.Fatalf("saw %d transactions, want %d", offset, transactionCount)
	}
}

func TestBatchTransactionsRejectsOversizedTransaction(t *testing.T) {
	transactions := []client.Transaction{{Notes: strings.Repeat("x", 1000), ImportedID: "large"}}
	opts := client.ImportTransactionsOptions{}
	requestBytes, err := client.ImportTransactionsRequestSize(transactions, opts)
	if err != nil {
		t.Fatalf("measuring request: %v", err)
	}

	_, err = batchTransactions(transactions, opts, requestBytes-1, 100)
	if err == nil {
		t.Fatal("expected oversized transaction error")
	}
	want := fmt.Sprintf("transaction 1 (import ID %q) requires %d bytes, maximum is %d", "large", requestBytes, requestBytes-1)
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %q, want it to contain %q", err, want)
	}
}

func TestBatchTransactionsReportsLaterOversizedTransaction(t *testing.T) {
	transactions := []client.Transaction{
		{ImportedID: "small"},
		{Notes: strings.Repeat("x", 1000), ImportedID: "large"},
	}
	opts := client.ImportTransactionsOptions{}
	maxRequestBytes, err := client.ImportTransactionsRequestSize(transactions[:1], opts)
	if err != nil {
		t.Fatalf("measuring small transaction: %v", err)
	}

	batches, err := batchTransactions(transactions, opts, maxRequestBytes, 10)
	if err == nil {
		t.Fatal("expected oversized transaction error")
	}
	if batches != nil {
		t.Fatalf("expected no partial batch plan, got %+v", batches)
	}
	for _, want := range []string{"transaction 2", `import ID "large"`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want it to contain %q", err, want)
		}
	}
}

func TestBatchTransactionsEmptyInput(t *testing.T) {
	batches, err := batchTransactions(nil, client.ImportTransactionsOptions{}, 1, 1)
	if err != nil {
		t.Fatalf("batchTransactions() error = %v", err)
	}
	if len(batches) != 0 {
		t.Fatalf("got %d batches, want none", len(batches))
	}
}

func TestBatchTransactionsValidatesLimits(t *testing.T) {
	tests := []struct {
		name            string
		maxRequestBytes int
		batchSize       int
	}{
		{name: "zero bytes", maxRequestBytes: 0, batchSize: 1},
		{name: "negative bytes", maxRequestBytes: -1, batchSize: 1},
		{name: "zero batch size", maxRequestBytes: 1, batchSize: 0},
		{name: "negative batch size", maxRequestBytes: 1, batchSize: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := batchTransactions(nil, client.ImportTransactionsOptions{}, tt.maxRequestBytes, tt.batchSize); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
