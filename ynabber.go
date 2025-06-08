package ynabber

import (
	"context"
	"log/slog"

	"golang.org/x/sync/errgroup"
)

type Ynabber struct {
	Readers []Reader
	Writers []Writer

	config *Config
	logger slog.Logger
}

// NewYnabber creates a new Ynabber instance
func NewYnabber(config *Config) *Ynabber {
	return &Ynabber{
		config: config,
		logger: *slog.Default(),
	}
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
// fan out to all writers. Returns immediately on first error from any reader or
// writer.
func (y *Ynabber) Run() error {
	g, ctx := errgroup.WithContext(context.Background())

	// Move transactions from reader to writer in batches on this channel.
	// Multiple readers and writer can be used
	batches := make(chan []Transaction)

	// Create a channel for each writer and fan out transactions to each one
	channels := make([]chan []Transaction, len(y.Writers))
	for c := range channels {
		channels[c] = make(chan []Transaction)
	}

	// Fan out transactions to all writer channels
	g.Go(func() error {
		defer func() {
			close(batches)
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
	for c, writer := range y.Writers {
		g.Go(func() error {
			return writer.Runner(ctx, channels[c])
		})
	}

	// Start all readers
	for _, reader := range y.Readers {
		g.Go(func() error {
			return reader.Runner(ctx, batches)
		})
	}

	// Wait for all goroutines to complete or first error
	if err := g.Wait(); err != nil && err != context.Canceled {
		return err
	}

	y.logger.Info("all readers and writers completed successfully")
	return nil
}
