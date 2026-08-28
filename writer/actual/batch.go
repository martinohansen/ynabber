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
//
// Two invariants keep the loop below correct:
//
//   - i-first is the number of transactions already accumulated into the
//     current batch. That makes it both the count limit and the test for
//     whether an element separator is needed.
//   - No batch is ever empty, so every flush emits at least one transaction.
//     This depends on the oversized-transaction check staying ahead of the
//     flush decision: because a lone transaction is known to fit, the byte
//     limit can never force a flush while i == first.
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

	// A JSON request is a fixed envelope around an array. Measuring the empty
	// request once and each one-transaction request once gives the exact
	// additive size of every array element; only a comma byte is added between
	// elements. This keeps planning linear without duplicating the client's JSON
	// schema in the writer. The empty slice must stay non-nil, because a nil
	// slice encodes as null rather than [] and would shift every element's size.
	// client.TestImportTransactionsRequestSizeIsAdditive guards the identity.
	emptyRequestBytes, err := client.ImportTransactionsRequestSize([]client.Transaction{}, opts)
	if err != nil {
		return nil, fmt.Errorf("measuring empty import request: %w", err)
	}

	var batches []transactionBatch
	first := 0
	batchBytes := emptyRequestBytes
	seenImportIDs := make(map[string]struct{})
	for i := range transactions {
		singleRequestBytes, err := client.ImportTransactionsRequestSize(transactions[i:i+1], opts)
		if err != nil {
			return nil, fmt.Errorf("measuring transaction %d: %w", i+1, err)
		}
		if singleRequestBytes > maxRequestBytes {
			return nil, oversizedTransactionError(i, transactions[i], singleRequestBytes, maxRequestBytes)
		}

		transactionBytes := singleRequestBytes - emptyRequestBytes
		separatorBytes := 0
		if i > first {
			separatorBytes = 1
		}
		// seenImportIDs only ever receives non-empty IDs, so an empty
		// imported_id can never register as a duplicate.
		_, duplicateImportID := seenImportIDs[transactions[i].ImportedID]

		if i-first == batchSize || duplicateImportID || batchBytes+separatorBytes+transactionBytes > maxRequestBytes {
			batches = append(batches, newTransactionBatch(transactions, first, i, batchBytes))
			first = i
			batchBytes = emptyRequestBytes
			separatorBytes = 0
			clear(seenImportIDs)
		}

		batchBytes += separatorBytes + transactionBytes
		if transactions[i].ImportedID != "" {
			seenImportIDs[transactions[i].ImportedID] = struct{}{}
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
