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
	}

	// YNAB related settings
	YNAB struct {
		// PayeeStrip is a list of words to remove from the Payee before sending
		// to YNAB. For example: "foo,bar"
		PayeeStrip []string `envconfig:"YNABBER_PAYEE_STRIP"`
	}
}
