package ynabber

import (
	"time"
)

//go:generate go run ./cmd/gendocs -file config.go -file reader/*/config.go -file writer/*/config.go -o CONFIGURATION.md

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
}
