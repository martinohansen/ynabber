package ynabber

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"golang.org/x/sync/errgroup"
)

type Ynabber struct {
	readers []Reader
	writers []Writer
	logger  *slog.Logger
}

// New creates a Ynabber pipeline from caller-owned readers and writers.
func New(readers []Reader, writers []Writer, logger *slog.Logger) (*Ynabber, error) {
	if len(readers) == 0 {
		return nil, fmt.Errorf("ynabber: at least one reader is required")
	}
	if len(writers) == 0 {
		return nil, fmt.Errorf("ynabber: at least one writer is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Ynabber{
		readers: append([]Reader(nil), readers...),
		writers: append([]Writer(nil), writers...),
		logger:  logger,
	}, nil
}

type Reader interface {
	Runner(ctx context.Context, out chan<- []Transaction) error
	String() string
}

type Writer interface {
	Runner(ctx context.Context, in <-chan []Transaction) error
	String() string
}

// Run starts Ynabber by reading transactions from all readers into a channel to
// fan out to all writers. An error cancels the other pipeline components.
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

	y.logger.Info("all readers and writers completed successfully")
	return nil
}
