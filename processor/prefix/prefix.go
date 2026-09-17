package prefix

import (
	"context"
	"slices"

	"github.com/kelseyhightower/envconfig"
	"github.com/martinohansen/ynabber"
)

// Processor prepends a configured string to every payee, including empty ones.
type Processor struct {
	Config Config
}

// NewProcessor loads configuration from environment variables.
func NewProcessor() (*Processor, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}
	return &Processor{Config: cfg}, nil
}

func (p Processor) String() string { return "prefix" }

// Process returns a transformed copy and leaves the input batch unchanged.
func (p Processor) Process(ctx context.Context, transactions []ynabber.Transaction) ([]ynabber.Transaction, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result := slices.Clone(transactions)
	for i := range result {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		result[i].Payee = p.Config.Prefix + result[i].Payee
	}
	return result, nil
}
