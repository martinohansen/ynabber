// Package json implements a writer that outputs transactions as JSON to stdout.
// This writer is useful for debugging, testing, and integration with other systems
// that can consume JSON-formatted transaction data.
package json

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/martinohansen/ynabber"
)

type Writer struct{}

func (w Writer) String() string {
	return "json"
}

func (w Writer) Bulk(tx []ynabber.Transaction) error {
	b, err := json.MarshalIndent(tx, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling: %w", err)
	}
	fmt.Println(string(b))
	return nil
}

func (w Writer) Runner(ctx context.Context, in <-chan []ynabber.Transaction) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case batch, ok := <-in:
			if !ok {
				return nil // Channel closed, normal termination
			}
			if err := w.Bulk(batch); err != nil {
				return err
			}
		}
	}
}
