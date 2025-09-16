// Package actual provides a writer implementation that sends transactions to
// an Actual Budget HTTP API instance.
package actual

import (
	"encoding/json"
	"fmt"
	"time"
)

type Date time.Time

const dateLayout = "2006-01-02"

// Decode implements envconfig.Decoder allowing the date to be provided as a
// YYYY-MM-DD string in environment variables.
func (d *Date) Decode(value string) error {
	if value == "" {
		*d = Date(time.Time{})
		return nil
	}
	parsed, err := time.Parse(dateLayout, value)
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

	// AccountMap maps IBAN numbers to Actual account identifiers.
	AccountMap AccountMap `envconfig:"ACTUAL_ACCOUNTMAP"`

	// EncryptionPassword optionally unlocks end-to-end encrypted budgets.
	EncryptionPassword string `envconfig:"ACTUAL_ENCRYPTION_PASSWORD"`

	// FromDate skips transactions older than this date when provided.
	FromDate Date `envconfig:"ACTUAL_FROM_DATE"`

	// Delay prevents sending very recent transactions that might still change.
	Delay time.Duration `envconfig:"ACTUAL_DELAY" default:"0"`

	// RunTransfers instructs Actual to automatically create transfers.
	RunTransfers bool `envconfig:"ACTUAL_RUN_TRANSFERS" default:"false"`

	// LearnCategories updates rules based on the incoming category field.
	LearnCategories bool `envconfig:"ACTUAL_LEARN_CATEGORIES" default:"false"`

	// Cleared toggles the cleared flag for newly created transactions.
	Cleared bool `envconfig:"ACTUAL_CLEARED" default:"false"`
}
