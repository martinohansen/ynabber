package ynabber

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"
)

// Mock reader that runs once and exits (simulates interval=0)
type mockOneShotReader struct {
	data []Transaction
}

type waitingReader struct{}

func (waitingReader) String() string { return "waiting-reader" }
func (waitingReader) Runner(ctx context.Context, _ chan<- []Transaction) error {
	<-ctx.Done()
	return ctx.Err()
}

type errorReader struct {
	err error
}

func (errorReader) String() string { return "error-reader" }
func (r errorReader) Runner(context.Context, chan<- []Transaction) error {
	return r.err
}

type errorWriter struct {
	err error
}

func (errorWriter) String() string { return "error-writer" }
func (w errorWriter) Runner(context.Context, <-chan []Transaction) error {
	return w.err
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
	y, err := New([]Reader{reader}, []Writer{writer}, slog.Default())
	if err != nil {
		t.Fatal(err)
	}

	// Run with timeout to ensure it doesn't hang
	done := make(chan error, 1)
	go func() {
		done <- y.Run(context.Background())
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

func TestNewRequiresComponents(t *testing.T) {
	if _, err := New(nil, []Writer{&mockWriter{}}, nil); err == nil {
		t.Fatal("New() accepted no readers")
	}
	if _, err := New([]Reader{&mockOneShotReader{}}, nil, nil); err == nil {
		t.Fatal("New() accepted no writers")
	}
}

func TestNewCopiesComponents(t *testing.T) {
	reader := &mockOneShotReader{}
	writer := &mockWriter{}
	readers := []Reader{reader}
	writers := []Writer{writer}
	y, err := New(readers, writers, nil)
	if err != nil {
		t.Fatal(err)
	}
	readers[0] = errorReader{err: errors.New("replacement reader")}
	writers[0] = errorWriter{err: errors.New("replacement writer")}
	if y.readers[0] != reader || y.writers[0] != writer {
		t.Fatal("New() retained caller-owned component slices")
	}
	if y.logger == nil {
		t.Fatal("New() did not use the default logger")
	}
}

func TestRunReturnsComponentErrors(t *testing.T) {
	tests := []struct {
		name    string
		readers []Reader
		writers []Writer
	}{
		{
			name:    "reader",
			readers: []Reader{errorReader{}},
			writers: []Writer{&mockWriter{}},
		},
		{
			name:    "writer",
			readers: []Reader{waitingReader{}},
			writers: []Writer{errorWriter{}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wantErr := errors.New(test.name + " failed")
			switch test.name {
			case "reader":
				test.readers[0] = errorReader{err: wantErr}
			case "writer":
				test.writers[0] = errorWriter{err: wantErr}
			}
			y, err := New(test.readers, test.writers, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := y.Run(context.Background()); !errors.Is(err, wantErr) {
				t.Fatalf("Run() error = %v, want %v", err, wantErr)
			}
		})
	}
}

func TestRunPreservesCallerCancellation(t *testing.T) {
	y, err := New([]Reader{waitingReader{}}, []Writer{&mockWriter{}}, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := y.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context cancellation", err)
	}
}
