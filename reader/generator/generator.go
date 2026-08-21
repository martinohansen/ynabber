package generator

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/martinohansen/ynabber"
	"github.com/martinohansen/ynabber/internal/web"
)

// Config for the generator reader
type Config struct {
	// Interval between generating new transactions
	Interval time.Duration `envconfig:"YNABBER_GENERATOR_INTERVAL" default:"5s"`

	// BatchSize is the number of transactions to generate in each bulk
	BatchSize int `envconfig:"YNABBER_GENERATOR_BULK_SIZE" default:"2"`
}

// Reader generates random transactions for testing purposes.
type Reader struct {
	Config Config
	logger *slog.Logger

	mu             sync.Mutex
	totalGenerated int
	lastBatch      int
	lastRun        time.Time
}

// NewReader creates a new generator
func NewReader() (*Reader, error) {
	var cfg Config
	err := envconfig.Process("", &cfg)
	if err != nil {
		return nil, err
	}

	return &Reader{
		Config: cfg,
		logger: slog.Default().With("reader", "generator"),
	}, nil
}

// Name implements web.Component.
func (r *Reader) Name() string { return "generator" }

// String implements fmt.Stringer (used by ynabber.Reader).
func (r *Reader) String() string { return "generator" }

// Status implements web.StatusProvider.
func (r *Reader) Status() web.ComponentStatus {
	r.mu.Lock()
	defer r.mu.Unlock()

	lastRun := "never"
	if !r.lastRun.IsZero() {
		lastRun = r.lastRun.Format(time.RFC3339)
	}

	return web.ComponentStatus{
		Healthy: r.totalGenerated > 0 || r.lastRun.IsZero(),
		Summary: "Generates synthetic transactions for testing",
		Fields: []web.StatusField{
			{Label: "interval", Value: r.Config.Interval.String()},
			{Label: "batch size", Value: fmt.Sprintf("%d", r.Config.BatchSize)},
			{Label: "total generated", Value: fmt.Sprintf("%d", r.totalGenerated)},
			{Label: "last batch", Value: fmt.Sprintf("%d", r.lastBatch)},
			{Label: "last run", Value: lastRun},
		},
	}
}

// Bulk generates a batch of random transactions.
func (r *Reader) Bulk() ([]ynabber.Transaction, error) {
	account := ynabber.Account{
		ID:   ynabber.ID("GB33BUKB20201555555555"),
		Name: "Checking Account",
		IBAN: "GB33BUKB20201555555555",
	}

	payees := []string{
		"Grocery Store", "Gas Station", "Coffee Shop", "Restaurant",
		"Online Store", "Utility Company", "Bank Transfer", "ATM Withdrawal",
		"Pharmacy", "Bookstore", "Movie Theater", "Subscription Service",
	}

	memos := []string{
		"Weekly groceries", "Fuel", "Morning coffee", "Lunch meeting",
		"Online purchase", "Electric bill", "Savings transfer", "Cash withdrawal",
		"Prescription", "Technical book", "Weekend movie", "Monthly subscription",
	}

	transactions := make([]ynabber.Transaction, r.Config.BatchSize)
	for i := 0; i < r.Config.BatchSize; i++ {
		transactions[i] = ynabber.Transaction{
			Account: account,
			ID:      ynabber.ID(fmt.Sprintf("gen_%d_%d", time.Now().UnixNano(), i)),
			Date:    time.Now().UTC(),
			Payee:   payees[rand.Intn(len(payees))],
			Memo:    memos[rand.Intn(len(memos))],
			Amount:  ynabber.Milliunits(rand.Intn(1000000) - 500000),
		}
	}

	r.mu.Lock()
	r.totalGenerated += len(transactions)
	r.lastBatch = len(transactions)
	r.lastRun = time.Now()
	r.mu.Unlock()

	return transactions, nil
}

// Runner continuously generates transactions and sends them to out.
func (r *Reader) Runner(ctx context.Context, out chan<- []ynabber.Transaction) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		batch, err := r.Bulk()
		if err != nil {
			r.logger.Error("generating transaction", "error", err)
			return err
		}

		r.logger.Info("generated transaction(s)", "count", len(batch))

		select {
		case out <- batch:
		case <-ctx.Done():
			return ctx.Err()
		}

		r.logger.Info("waiting for next run", "in", r.Config.Interval)

		select {
		case <-time.After(r.Config.Interval):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
