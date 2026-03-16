// Package json implements a writer that outputs transactions as JSON to stdout.
// This writer is useful for debugging, testing, and integration with other systems
// that can consume JSON-formatted transaction data.
package json

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/martinohansen/ynabber"
	"github.com/martinohansen/ynabber/internal/web"
)

// Writer outputs transactions as JSON to stdout.
type Writer struct {
	mu           sync.Mutex
	totalWritten int
	lastBatch    int
	lastRun      time.Time
}

// Name implements web.Component.
func (w *Writer) Name() string { return "json" }

// String implements fmt.Stringer (used by ynabber.Writer).
func (w *Writer) String() string { return "json" }

// Status implements web.StatusProvider.
func (w *Writer) Status() web.ComponentStatus {
	w.mu.Lock()
	defer w.mu.Unlock()

	lastRun := "never"
	if !w.lastRun.IsZero() {
		lastRun = w.lastRun.Format(time.RFC3339)
	}

	return web.ComponentStatus{
		Healthy: true,
		Summary: "Writes transactions as JSON to stdout",
		Fields: []web.StatusField{
			{Label: "total written", Value: fmt.Sprintf("%d", w.totalWritten)},
			{Label: "last batch", Value: fmt.Sprintf("%d", w.lastBatch)},
			{Label: "last run", Value: lastRun},
		},
	}
}

// Bulk writes a batch of transactions as indented JSON to stdout.
func (w *Writer) Bulk(tx []ynabber.Transaction) error {
	b, err := json.MarshalIndent(tx, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling: %w", err)
	}
	fmt.Println(string(b))

	w.mu.Lock()
	w.totalWritten += len(tx)
	w.lastBatch = len(tx)
	w.lastRun = time.Now()
	w.mu.Unlock()

	return nil
}

// Runner receives transaction batches and writes each one to stdout.
func (w *Writer) Runner(ctx context.Context, in <-chan []ynabber.Transaction) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case batch, ok := <-in:
			if !ok {
				return nil // channel closed, normal termination
			}
			if err := w.Bulk(batch); err != nil {
				return err
			}
		}
	}
}
