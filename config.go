package ynabber

import (
	"encoding/json"
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

// Config is loaded from the environment during execution with cmd/ynabber
type Config struct {
	// DataDir is the path for storing files e.g. Nordigen authorization
	DataDir string `envconfig:"YNABBER_DATADIR" default:"."`

	// Debug prints more log statements
	Debug bool `envconfig:"YNABBER_DEBUG" default:"false"`

	// Interval is how often to execute the read/write loop, 0=run only once
	Interval time.Duration `envconfig:"YNABBER_INTERVAL" default:"5m"`

	// Readers is a list of sources to read transactions from. Currently only
	// Nordigen is supported.
	Readers []string `envconfig:"YNABBER_READERS" default:"nordigen"`

	// Writers is a list of destinations to write transactions from. Currently
	// only YNAB is supported.
	Writers []string `envconfig:"YNABBER_WRITERS" default:"ynab"`

	// PayeeStrip is depreciated please use Nordigen.PayeeStrip instead
	PayeeStrip []string `envconfig:"YNABBER_PAYEE_STRIP"`

	// Reader and/or writer specific settings
	Nordigen Nordigen
	YNAB     YNAB
}

// Nordigen related settings
type Nordigen struct {
	// AccountMap of Nordigen account IDs to YNAB account IDs in JSON. For
	// example: '{"<nordigen account id>": "<ynab account id>"}'
	AccountMap AccountMap `envconfig:"NORDIGEN_ACCOUNTMAP"`

	// BankID is used to create requisition
	BankID string `envconfig:"NORDIGEN_BANKID"`

	// SecretID is used to create requisition
	SecretID string `envconfig:"NORDIGEN_SECRET_ID"`

	// SecretKey is used to create requisition
	SecretKey string `envconfig:"NORDIGEN_SECRET_KEY"`

	// Use named datafile(relative path in datadir, absolute if starts with slash) instead of default (ynabber-NORDIGEN_BANKID.json)
	Datafile string `envconfig:"NORDIGEN_DATAFILE"`

	// PayeeSource is a list of sources for Payee candidates, the first
	// method that yields a result will be used. Valid options are:
	// unstructured and name.
	//
	// Option unstructured equals to the `RemittanceInformationUnstructured`
	// filed from Nordigen while name equals either `debtorName` or
	// `creditorName`.
	PayeeSource []string `envconfig:"NORDIGEN_PAYEE_SOURCE" default:"unstructured,name"`

	// PayeeStrip is a list of words to remove from Payee. For example:
	// "foo,bar"
	PayeeStrip []string `envconfig:"NORDIGEN_PAYEE_STRIP"`
}

// YNAB related settings
type YNAB struct {
	// BudgetID for the budget you want to import transactions into. You can
	// find the ID in the URL of YNAB: https://app.youneedabudget.com/<budget_id>/budget
	BudgetID string `envconfig:"YNAB_BUDGETID"`

	// Token is your personal access token as obtained from the YNAB developer
	// settings section
	Token string `envconfig:"YNAB_TOKEN"`

	// ImportFromDate only import transactions from this date and onward. For
	// FromDate only import transactions from this date and onward. For
	// example: 2006-01-02
	ImportFromDate Date `envconfig:"YNAB_IMPORT_FROM_DATE"`

	// Set cleared status, possible values: cleared, uncleared, reconciled .
	// Default is uncleared for historical reasons but recommend setting this
	// to cleared because ynabber transactions are cleared by bank.
	// They'd still be unapproved until approved in YNAB.
	Cleared string `envconfig:"YNAB_CLEARED" default:"uncleared"`

	ImportID ImportID
}

// ImportID can be either v1 or v2. All new users should use v2 because it
// have a lower potability of making duplicate transactions. But v1 remains
// the default to retain backwards compatibility.
//
// To migrate from v1 to v2 simply set the v2 to any date and all transactions
// from and including that date will be using v2 of the import ID generator.
type ImportID struct {
	// V1 will be used from this date
	V1 Date `envconfig:"YNAB_IMPORT_ID_V1" default:"1970-01-01"`

	// V2 will be used from this date, for example: 2022-12-24
	V2 Date `envconfig:"YNAB_IMPORT_ID_V2" default:"9999-01-01"`
}
