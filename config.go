package ynabber

import (
	"time"
)

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

	// Nordigen related settings
	Nordigen struct {
		// AccountMap of Nordigen account IDs to YNAB account IDs in JSON. For
		// example: '{"<nordigen account id>": "<ynab account id>"}'
		AccountMap AccountMap `envconfig:"NORDIGEN_ACCOUNTMAP"`

		// BankID is used to create requisition
		BankID string `envconfig:"NORDIGEN_BANKID"`

		// SecretID is used to create requisition
		SecretID string `envconfig:"NORDIGEN_SECRET_ID"`

		// SecretKey is used to create requisition
		SecretKey string `envconfig:"NORDIGEN_SECRET_KEY"`

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
	YNAB struct {
		// PayeeStrip is depreciated please use Nordigen.PayeeStrip instead
		PayeeStrip []string `envconfig:"YNABBER_PAYEE_STRIP"`
	}
}
