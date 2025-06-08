// YNAB writes transactions You Need a Budget (YNAB) using their API. It handles
// transaction and account mapping, validation, deduplication, inflow/outflow
// swapping, and transaction filtering.
package ynab

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const DateFormat = "2006-01-02"

type Date time.Time

// Decode implements `envconfig.Decoder` for Date to parse string to time.Time
func (date *Date) Decode(value string) error {
	time, err := time.Parse(DateFormat, value)
	if err != nil {
		return err
	}
	*date = Date(time)
	return nil
}

type AccountMap map[string]string

// Decode implements `envconfig.Decoder` for AccountMap to decode JSON properly
func (accountMap *AccountMap) Decode(value string) error {
	err := json.Unmarshal([]byte(value), &accountMap)
	if err != nil {
		return err
	}
	return nil
}

type TransactionStatus string

const (
	Cleared    TransactionStatus = "cleared"
	Uncleared  TransactionStatus = "uncleared"
	Reconciled TransactionStatus = "reconciled"
)

// Decode implements `envconfig.Decoder` for TransactionStatus
func (cs *TransactionStatus) Decode(value string) error {
	lowered := strings.ToLower(value)
	switch lowered {
	case string(Cleared), string(Uncleared), string(Reconciled):
		*cs = TransactionStatus(lowered)
		return nil
	default:
		return fmt.Errorf("unknown value %s", value)
	}
}

func (cs TransactionStatus) String() string {
	return string(cs)
}

type Config struct {
	// BudgetID for the budget you want to import transactions into. You can
	// find the ID in the URL of YNAB: https://app.youneedabudget.com/<budget_id>/budget
	BudgetID string `envconfig:"YNAB_BUDGETID"`

	// Token is your personal access token as obtained from the YNAB developer
	// settings section
	Token string `envconfig:"YNAB_TOKEN"`

	// AccountMap of IBAN to YNAB account IDs in JSON. For example:
	// '{"<IBAN>": "<YNAB Account ID>"}'
	AccountMap AccountMap `envconfig:"YNAB_ACCOUNTMAP"`

	// FromDate only import transactions from this date and onward. For
	// example: 2006-01-02
	FromDate Date `envconfig:"YNAB_FROM_DATE"`

	// Delay sending transaction to YNAB by this duration. This can be necessary
	// if the bank changes transaction IDs after some time. Default is 0 (no
	// delay).
	Delay time.Duration `envconfig:"YNAB_DELAY" default:"0"`

	// Set cleared status, possible values: cleared, uncleared, reconciled .
	// Default is uncleared for historical reasons but recommend setting this
	// to cleared because ynabber transactions are cleared by bank.
	// They'd still be unapproved until approved in YNAB.
	Cleared TransactionStatus `envconfig:"YNAB_CLEARED" default:"uncleared"`

	// SwapFlow changes inflow to outflow and vice versa for any account with a
	// IBAN number in the list. This maybe be relevant for credit card accounts.
	//
	// Example: "DK9520000123456789,NO8330001234567"
	SwapFlow []string `envconfig:"YNAB_SWAPFLOW"`
}
