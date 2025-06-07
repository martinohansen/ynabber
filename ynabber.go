package ynabber

import (
	"log/slog"
	"sync"
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
	Bulk() ([]Transaction, error)
	Runner(out chan<- []Transaction, errCh chan<- error)
	String() string
}

type Writer interface {
	Bulk([]Transaction) error
	String() string
}

// Run starts Ynabber by reading transactions from all readers into a channel to
// fan out to all writers
func (y *Ynabber) Run() error {
	// Move transactions from reader to writer in batches on this channel.
	// Multiple readers and writer can be used
	batches := make(chan []Transaction)
	defer close(batches)

	// Create a channel for each writer and fan out transactions to each one
	channels := make([]chan []Transaction, len(y.Writers))
	for c := range channels {
		channels[c] = make(chan []Transaction)
	}
	go func() {
		for batch := range batches {
			for _, c := range channels {
				c <- batch
			}
		}
	}()

	// Create a channel to collect errors from readers and writers
	errCh := make(chan error, len(y.Readers)+len(y.Writers))
	defer close(errCh)

	for c, writer := range y.Writers {
		go func(writer Writer, batches <-chan []Transaction) {
			for batch := range batches {
				if err := writer.Bulk(batch); err != nil {
					y.logger.Error("writing", "error", err, "writer", writer)
				}
			}
		}(writer, channels[c])
	}

	var wg sync.WaitGroup
	for _, r := range y.Readers {
		wg.Add(1)
		go func(reader Reader) {
			defer wg.Done()
			reader.Runner(batches, errCh)
		}(r)
	}
	wg.Wait() // Wait until all readers exit (interval=0) or end due to other reasons

	// Close all writer channels to signal completion
	for _, c := range channels {
		close(c)
	}

	// Check for any errors that occurred during processing
	select {
	case err := <-errCh:
		return err
	default:
		y.logger.Info("all readers done")
		return nil
	}
}
