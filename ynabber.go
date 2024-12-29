package ynabber

import (
	"log/slog"
	"strconv"
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
}

type Writer interface {
	Bulk([]Transaction) error
}

type Account struct {
	ID   ID
	Name string
	IBAN string
}

type ID string

type Payee string

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
	Payee  Payee      `json:"payee"`
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
func (y *Ynabber) Run() {
	batches := make(chan []Transaction)

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

	for c, writer := range y.Writers {
		go func(writer Writer, batches <-chan []Transaction) {
			for batch := range batches {
				err := writer.Bulk(batch)
				if err != nil {
					y.logger.Error("writing", "error", err, "writer", writer)
				}
			}
		}(writer, channels[c])
	}

	for _, r := range y.Readers {
		go func(reader Reader) {
			for {
				start := time.Now()
				batch, err := reader.Bulk()
				if err != nil {
					y.logger.Error("reading", "error", err, "reader", reader)
					continue
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

	select {}
}
