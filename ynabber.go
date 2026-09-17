package ynabber

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"golang.org/x/sync/errgroup"
)

type Ynabber struct {
	readers    []Reader
	writers    []Writer
	processors []Processor
	logger     *slog.Logger
}

// New creates a Ynabber pipeline from caller-owned readers and writers.
func New(readers []Reader, writers []Writer, logger *slog.Logger, options ...Option) (*Ynabber, error) {
	if len(readers) == 0 {
		return nil, fmt.Errorf("ynabber: at least one reader is required")
	}
	if len(writers) == 0 {
		return nil, fmt.Errorf("ynabber: at least one writer is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	y := &Ynabber{
		readers: append([]Reader(nil), readers...),
		writers: append([]Writer(nil), writers...),
		logger:  logger,
	}
	for _, option := range options {
		option(y)
	}
	return y, nil
}

// Option configures a pipeline at construction time.
type Option func(*Ynabber)

// WithProcessors appends processors in execution order. It copies the supplied
// slice so later changes to that slice cannot change pipeline configuration.
func WithProcessors(processors ...Processor) Option {
	processors = append([]Processor(nil), processors...)
	return func(y *Ynabber) {
		y.processors = append(y.processors, processors...)
	}
}

// Processor transforms payees and memos before transactions reach writers.
// Process must preserve transaction count, order, ID, account, date, and amount.
// It must leave its input unchanged and copy the batch before modifying it.
// Calls are sequential within a pipeline. Process must respect cancellation.
type Processor interface {
	Process(context.Context, []Transaction) ([]Transaction, error)
	String() string
}

type Reader interface {
	Runner(ctx context.Context, out chan<- []Transaction) error
	String() string
}

// Writer receives processed batches. It must treat them as read-only because
// the same batch is shared with every writer.
type Writer interface {
	Runner(ctx context.Context, in <-chan []Transaction) error
	String() string
}

// Run processes each reader batch in order before fan-out to all writers.
// An error cancels the other components. A failed processor batch is not sent
// to writers; previously delivered batches are not rolled back.
func (y *Ynabber) Run(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	// Move transactions from reader to writer in batches on this channel.
	// Multiple readers and writer can be used
	batches := make(chan []Transaction)

	// Create a channel for each writer and fan out transactions to each one
	channels := make([]chan []Transaction, len(y.writers))
	for c := range channels {
		channels[c] = make(chan []Transaction)
	}

	// Track when all readers are done
	var readerWg sync.WaitGroup
	readerWg.Add(len(y.readers))

	// Close batches channel when all readers are done
	go func() {
		readerWg.Wait()
		close(batches)
	}()

	// Fan out transactions to all writer channels
	g.Go(func() error {
		defer func() {
			for _, c := range channels {
				close(c)
			}
		}()
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case batch, ok := <-batches:
				if !ok {
					return nil
				}
				for _, processor := range y.processors {
					if err := ctx.Err(); err != nil {
						return err
					}
					var err error
					batch, err = processor.Process(ctx, batch)
					if err != nil {
						return fmt.Errorf("processor %s: %w", processor.String(), err)
					}
				}
				if err := ctx.Err(); err != nil {
					return err
				}
				for _, c := range channels {
					select {
					case c <- batch:
					case <-ctx.Done():
						return ctx.Err()
					}
				}
			}
		}
	})

	// Start all writers
	for c, writer := range y.writers {
		g.Go(func() error {
			return writer.Runner(ctx, channels[c])
		})
	}

	// Start all readers
	for _, reader := range y.readers {
		g.Go(func() error {
			defer readerWg.Done()
			return reader.Runner(ctx, batches)
		})
	}

	// Wait for all goroutines to complete or first error
	if err := g.Wait(); err != nil {
		return err
	}

	y.logger.Info("pipeline completed successfully")
	return nil
}
