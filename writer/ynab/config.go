// YNAB writes transactions You Need a Budget (YNAB) using their API. It handles
// transaction and account mapping, validation, deduplication, inflow/outflow
// swapping, and transaction filtering.
package ynab

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/martinohansen/ynabber"
)

// DateFormat re-exports ynabber.DateFormat for callers that import this package.
const DateFormat = ynabber.DateFormat

// Date re-exports ynabber.Date so existing code in this package requires no changes.
type Date = ynabber.Date

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

	// Token is your personal access token obtained from the YNAB developer
	// settings section
	Token string `envconfig:"YNAB_TOKEN"`

	// AccountMap maps account identifiers (ID or IBAN) to YNAB account IDs in JSON format.
	// It supports both IBAN (for nordigen or enablebanking standard accounts) and Account ID
	// (for enablebanking's account_uid). IBAN is preferred when the account has one, for
	// backward compatibility with Nordigen. Account ID is used only when IBAN is absent
	// (e.g. credit cards that EnableBanking does not expose an IBAN for).
	// Examples:
	// - With IBAN: '{"NO1234567890": "<YNAB Account ID>"}'
	// - With account_uid: '{"account-uid-123": "<YNAB Account ID>"}'
	// - Mixed: '{"NO1234567890": "<YNAB1>", "account-uid-123": "<YNAB2>"}'
	AccountMap AccountMap `envconfig:"YNAB_ACCOUNTMAP"`

	// FromDate only imports transactions from this date onward. For
	// example: 2006-01-02
	FromDate Date `envconfig:"YNAB_FROM_DATE"`

	// Delay sending transactions to YNAB by this duration. This can be
	// necessary if the bank changes transaction IDs after some time. Default is
	// 0 (no delay).
	Delay time.Duration `envconfig:"YNAB_DELAY" default:"0"`

	// Cleared sets the transaction status. Possible values: cleared, uncleared,
	// reconciled.
	Cleared TransactionStatus `envconfig:"YNAB_CLEARED" default:"cleared"`

	// SwapFlow reverses inflow to outflow and vice versa for any account
	// identified by ID (enablebanking's account_uid) or IBAN (nordigen) in the list.
	// This may be relevant for credit card accounts.
	//
	// Example: "DK9520000123456789,NO8330001234567,account-uid-123"
	SwapFlow []string `envconfig:"YNAB_SWAPFLOW"`
}
