// Package actual provides a writer implementation that sends transactions to
// an Actual Budget HTTP API instance.
package actual

import (
	"encoding/json"
	"fmt"
	"time"
)

type Date time.Time

// Decode implements envconfig.Decoder, parsing a YYYY-MM-DD string into Date.
// An empty value is treated as "no date set" and results in a zero time.Time,
// which is the documented way to disable the bound in isDateAllowed.
func (d *Date) Decode(value string) error {
	if value == "" {
		*d = Date(time.Time{})
		return nil
	}
	parsed, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return err
	}
	*d = Date(parsed)
	return nil
}

// Time converts the custom Date type back to time.Time.
func (d Date) Time() time.Time {
	return time.Time(d)
}

type AccountMap map[string]string

// Decode implements envconfig.Decoder for parsing the JSON encoded mapping
// coming from environment variables.
func (a *AccountMap) Decode(value string) error {
	if value == "" {
		*a = AccountMap{}
		return nil
	}
	if err := json.Unmarshal([]byte(value), a); err != nil {
		return fmt.Errorf("decoding account map: %w", err)
	}
	return nil
}

// Config drives how the Actual writer connects to the Actual HTTP API.
type Config struct {
	// BaseURL points to the running actual-http-api service, e.g. https://actual.example.com
	BaseURL string `envconfig:"ACTUAL_BASE_URL"`

	// APIKey is an optional shared secret that will be sent via the x-api-key header.
	APIKey string `envconfig:"ACTUAL_API_KEY"`

	// BudgetID is the Actual Sync ID for the budget to update.
	BudgetID string `envconfig:"ACTUAL_BUDGET_ID"`

	// AccountMap maps reader accounts to Actual accounts. See reader for more
	// details. For example: '{"<IBAN or Account ID>": "<Actual Account ID>"}'
	AccountMap AccountMap `envconfig:"ACTUAL_ACCOUNTMAP"`

	// EncryptionPassword optionally unlocks end-to-end encrypted budgets.
	EncryptionPassword string `envconfig:"ACTUAL_ENCRYPTION_PASSWORD"`

	// FromDate only imports transactions from this date onward. For
	// example: 2006-01-02
	FromDate Date `envconfig:"ACTUAL_FROM_DATE"`

	// Delay sending transactions to Actual by this duration. This can be
	// necessary if the bank changes transaction IDs after some time, or
	// enriches remittance information after booking (which can cause duplicate
	// imports). Default is 0 (no delay).
	Delay time.Duration `envconfig:"ACTUAL_DELAY" default:"0"`

	// Cleared sets the transaction cleared flag for newly created transactions.
	// Default is false.
	Cleared bool `envconfig:"ACTUAL_CLEARED" default:"false"`

	// ReimportDeleted controls whether Actual should reimport transactions that
	// were previously imported and then deleted. Default is false.
	ReimportDeleted bool `envconfig:"ACTUAL_REIMPORT_DELETED" default:"false"`

	// DryRun simulates the import without persisting any data. Useful for
	// verifying mappings and deduplication before writing. Default is false.
	DryRun bool `envconfig:"ACTUAL_DRY_RUN" default:"false"`
}
