package actual

import (
	"fmt"

	"github.com/martinohansen/ynabber/writer/actual/client"
)

type transactionBatch struct {
	transactions []client.Transaction
	requestBytes int
}

// batchTransactions partitions transactions without reordering them. A batch
// never contains the same non-empty imported_id twice because Actual may not
// reconcile duplicates within one import request. All batches are planned
// before the caller sends anything, so an invalid limit or an oversized
// transaction fails the account without causing a partial import.
func batchTransactions(transactions []client.Transaction, opts client.ImportTransactionsOptions, maxRequestBytes, batchSize int) ([]transactionBatch, error) {
	if maxRequestBytes <= 0 {
		return nil, fmt.Errorf("maximum request bytes must be greater than zero, got %d", maxRequestBytes)
	}
	if batchSize <= 0 {
		return nil, fmt.Errorf("batch size must be greater than zero, got %d", batchSize)
	}

	if len(transactions) == 0 {
		return nil, nil
	}

	var batches []transactionBatch
	first := 0
	batchBytes := 0
	seenImportIDs := make(map[string]struct{})
	for i := range transactions {
		importID := transactions[i].ImportedID
		_, duplicateImportID := seenImportIDs[importID]
		duplicateImportID = importID != "" && duplicateImportID
		if i-first == batchSize || duplicateImportID {
			batches = append(batches, newTransactionBatch(transactions, first, i, batchBytes))
			first = i
			clear(seenImportIDs)
		}

		candidateBytes, err := client.ImportTransactionsRequestSize(transactions[first:i+1], opts)
		if err != nil {
			return nil, fmt.Errorf("measuring batch ending at transaction %d: %w", i+1, err)
		}
		if candidateBytes > maxRequestBytes {
			if first == i {
				return nil, oversizedTransactionError(i, transactions[i], candidateBytes, maxRequestBytes)
			}

			batches = append(batches, newTransactionBatch(transactions, first, i, batchBytes))
			first = i
			clear(seenImportIDs)
			candidateBytes, err = client.ImportTransactionsRequestSize(transactions[i:i+1], opts)
			if err != nil {
				return nil, fmt.Errorf("measuring transaction %d: %w", i+1, err)
			}
			if candidateBytes > maxRequestBytes {
				return nil, oversizedTransactionError(i, transactions[i], candidateBytes, maxRequestBytes)
			}
		}

		batchBytes = candidateBytes
		if importID != "" {
			seenImportIDs[importID] = struct{}{}
		}
	}
	batches = append(batches, newTransactionBatch(transactions, first, len(transactions), batchBytes))

	return batches, nil
}

func newTransactionBatch(transactions []client.Transaction, first, end, requestBytes int) transactionBatch {
	return transactionBatch{
		// Restrict capacity so an append by a future importer cannot overwrite
		// transactions belonging to the next batch.
		transactions: transactions[first:end:end],
		requestBytes: requestBytes,
	}
}

func oversizedTransactionError(index int, transaction client.Transaction, requestBytes, maxRequestBytes int) error {
	if transaction.ImportedID != "" {
		return fmt.Errorf("transaction %d (import ID %q) requires %d bytes, maximum is %d", index+1, transaction.ImportedID, requestBytes, maxRequestBytes)
	}
	return fmt.Errorf("transaction %d requires %d bytes, maximum is %d", index+1, requestBytes, maxRequestBytes)
}
