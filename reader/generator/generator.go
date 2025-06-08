package generator

import (
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/martinohansen/ynabber"
)

// Config for the generator reader
type Config struct {
	// Interval between generating new transactions
	Interval time.Duration `envconfig:"YNABBER_GENERATOR_INTERVAL" default:"5s"`

	// BatchSize is the number of transactions to generate in each bulk
	BatchSize int `envconfig:"YNABBER_GENERATOR_BULK_SIZE" default:"2"`
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

// Reader generates random transactions for testing purposes
type Reader struct {
	Config Config
	logger *slog.Logger
}

func (r Reader) String() string {
	return "generator"
}

// Bulk generates transactions
func (r Reader) Bulk() ([]ynabber.Transaction, error) {
	account := ynabber.Account{
		ID:   ynabber.ID("GB33BUKB20201555555555"),
		Name: "Checking Account",
		IBAN: "GB33BUKB20201555555555",
	}

	// Sample payees and memos for realistic transactions
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
		// Random payee and memo
		payee := payees[rand.Intn(len(payees))]
		memo := memos[rand.Intn(len(memos))]

		// Random amount between -500.00 and 500.00 (in milliunits)
		amount := ynabber.Milliunits(rand.Intn(1000000) - 500000)

		// Use current time
		date := time.Now().UTC()

		transactions[i] = ynabber.Transaction{
			Account: account,
			ID:      ynabber.ID(fmt.Sprintf("gen_%d_%d", time.Now().UnixNano(), i)),
			Date:    date,
			Payee:   payee,
			Memo:    memo,
			Amount:  amount,
		}
	}

	return transactions, nil
}

// Runner continuously generates transactions and sends them to out
func (r Reader) Runner(out chan<- []ynabber.Transaction, errCh chan<- error) {
	for {
		batch, err := r.Bulk()
		if err != nil {
			r.logger.Error("generating transaction", "error", err)
			errCh <- err
			return
		}

		r.logger.Info("generated transaction(s)", "count", len(batch))
		out <- batch

		r.logger.Info("waiting for next run", "in", r.Config.Interval)
		time.Sleep(r.Config.Interval)
	}
}
