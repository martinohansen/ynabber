package ynabber

import (
	"errors"
	"log/slog"
	"strconv"
	"sync"
	"time"
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
	String() string
}

type Writer interface {
	Bulk([]Transaction) error
	String() string
}

type Account struct {
	ID   ID
	Name string
	IBAN string
}

type ID string

type Milliunits int64

// Negate changes the sign of m to the opposite
func (m Milliunits) Negate() Milliunits {
	return m * -1
}

type Transaction struct {
	Account Account `json:"account"`
	ID      ID      `json:"id"`
	// Date is the date of the transaction in UTC time
	Date   time.Time  `json:"date"`
	Payee  string     `json:"payee"`
	Memo   string     `json:"memo"`
	Amount Milliunits `json:"amount"`
}

func (m Milliunits) String() string {
	return strconv.FormatInt(int64(m), 10)
}

// MilliunitsFromAmount returns a transaction amount in YNABs milliunits format
func MilliunitsFromAmount(amount float64) Milliunits {
	return Milliunits(amount * 1000)
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

			for {
				start := time.Now()
				batch, err := reader.Bulk()
				if err != nil {
					y.logger.Error("reading", "error", err, "reader", reader)

					var rl *RateLimitError
					if errors.As(err, &rl) && y.config.Interval != 0 {
						// If rate limited and not in one-shot mode wait for the
						// longest between the configured interval and the retry
						// timer to avoid retrying too quickly and exiting for
						// transient errors.
						wait := y.config.Interval
						if rl.RetryAfter > 0 && rl.RetryAfter > wait {
							wait = rl.RetryAfter
							y.logger.Info("retrying after", "duration", wait)
						} else {
							y.logger.Info("waiting for next run", "in", wait)
						}
						time.Sleep(wait)
						continue
					}

					// Unrecoverable error, send it down the channel
					errCh <- err
					return

				}

				batches <- batch
				y.logger.Info("run succeeded", "in", time.Since(start))

				// TODO(Martin): The interval should be controlled by the
				// reader. We are only pausing the entire reader goroutine
				// because thats how the config option is implemented now.
				// Eventually we should move this option into the reader
				// allowing for multiple readers with different intervals.
				if y.config.Interval > 0 {
					y.logger.Info("waiting for next run", "in", y.config.Interval)
					time.Sleep(y.config.Interval)
				} else {
					break
				}
			}
		}(r)
	}
	wg.Wait() // Wait until all readers exit(interval=0) or end due to other reasons

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
