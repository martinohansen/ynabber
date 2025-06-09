package ynabber

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"
)

// Mock reader that runs once and exits (simulates interval=0)
type mockOneShotReader struct {
	data []Transaction
}

func (r *mockOneShotReader) String() string { return "mock-oneshot-reader" }

func (r *mockOneShotReader) Runner(ctx context.Context, out chan<- []Transaction) error {
	select {
	case out <- r.data:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil // Exit immediately (interval=0 behavior)
}

// Mock writer that captures received batches
type mockWriter struct {
	// batches stores all received transaction batches
	batches [][]Transaction
	mu      sync.Mutex
}

func (w *mockWriter) String() string { return "mock-writer" }

func (w *mockWriter) Runner(ctx context.Context, in <-chan []Transaction) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case batch, ok := <-in:
			if !ok {
				return nil // Channel closed, normal termination
			}
			w.mu.Lock()
			w.batches = append(w.batches, batch)
			w.mu.Unlock()
		}
	}
}

func (w *mockWriter) getBatches() [][]Transaction {
	w.mu.Lock()
	defer w.mu.Unlock()
	result := make([][]Transaction, len(w.batches))
	copy(result, w.batches)
	return result
}

func TestOneShotBehavior(t *testing.T) {
	testTx := []Transaction{{
		Account: Account{
			ID:   "test-id",
			Name: "test-account",
			IBAN: "test-iban",
		},
		Payee:  "test-payee",
		Amount: Milliunits(1000),
		Date:   time.Now().UTC(),
	}}

	// Create mock reader and writer
	reader := &mockOneShotReader{data: testTx}
	writer := &mockWriter{}

	// Create ynabber instance
	y := &Ynabber{
		Readers: []Reader{reader},
		Writers: []Writer{writer},
		logger:  *slog.Default(),
	}

	// Run with timeout to ensure it doesn't hang
	done := make(chan error, 1)
	go func() {
		done <- y.Run()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("	Run() failed: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Run() timed out")
	}

	// Verify writer received the batch
	batches := writer.getBatches()
	if len(batches) != 1 {
		t.Fatalf("expected 1 batch, got %d", len(batches))
	}
	if len(batches[0]) != 1 {
		t.Fatalf("expected 1 transaction in batch, got %d", len(batches[0]))
	}
}
