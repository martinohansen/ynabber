package ynabber

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

// Config is loaded from the environment during execution with cmd/ynabber
type Config struct {
	// DataDir is the path for storing files
	DataDir string `envconfig:"YNABBER_DATADIR" default:"."`

	// Debug prints more log statements
	Debug bool `envconfig:"YNABBER_DEBUG" default:"false"`

	// Interval is how often to execute the read/write loop, 0=run only once
	Interval time.Duration `envconfig:"YNABBER_INTERVAL" default:"6h"`

	// Readers is a list of sources to read transactions from. Currently only
	// Nordigen is supported.
	Readers []string `envconfig:"YNABBER_READERS" default:"nordigen"`

	// Writers is a list of destinations to write transactions to.
	Writers []string `envconfig:"YNABBER_WRITERS" default:"ynab"`

	// Reader and/or writer specific settings
	Nordigen Nordigen
	YNAB     YNAB
}

// Nordigen related settings
type Nordigen struct {
	// BankID is used to create requisition
	BankID string `envconfig:"NORDIGEN_BANKID"`

	// SecretID is used to create requisition
	SecretID string `envconfig:"NORDIGEN_SECRET_ID"`

	// SecretKey is used to create requisition
	SecretKey string `envconfig:"NORDIGEN_SECRET_KEY"`

	// PayeeSource is a list of sources for Payee candidates, the first method
	// that yields a result will be used. Valid options are: unstructured, name
	// and additional.
	//
	//	* unstructured: uses the `RemittanceInformationUnstructured` field
	//	* name: uses either the either `debtorName` or `creditorName` field
	//	* additional: uses the `AdditionalInformation` field
	PayeeSource []string `envconfig:"NORDIGEN_PAYEE_SOURCE" default:"unstructured,name,additional"`

	// PayeeStrip is a list of words to remove from Payee. For example:
	// "foo,bar"
	PayeeStrip []string `envconfig:"NORDIGEN_PAYEE_STRIP"`

	// TransactionID is the field to use as transaction ID. Not all banks use
	// the same field and some even change the ID over time.
	//
	// Valid options are: TransactionId, InternalTransactionId
	TransactionID string `envconfig:"NORDIGEN_TRANSACTION_ID" default:"TransactionId"`

	// RequisitionHook is a exec hook thats executed at various stages of the
	// requisition process. The hook is executed with the following arguments:
	// <status> <link>
	RequisitionHook string `envconfig:"NORDIGEN_REQUISITION_HOOK"`

	// RequisitionFile overrides the file used to store the requisition. This
	// file is placed inside the YNABBER_DATADIR.
	RequisitionFile string `envconfig:"NORDIGEN_REQUISITION_FILE"`
}

// YNAB related settings
type YNAB struct {
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
