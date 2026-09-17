// Ynabber reads transactions, applies optional processors, and sends each
// processed batch to every writer.
package ynabber

//go:generate go run ./cmd/gendocs -file config.go -file reader/*/config.go -file processor/*/config.go -file writer/*/config.go -o CONFIGURATION.md

type Config struct {
	// DataDir is the path for storing files
	DataDir string `envconfig:"YNABBER_DATADIR" default:"."`

	// LogLevel sets the logging level (error, warn, info, debug, trace)
	LogLevel string `envconfig:"YNABBER_LOG_LEVEL" default:"info"`

	// LogFormat sets the logging format (text, json)
	LogFormat string `envconfig:"YNABBER_LOG_FORMAT" default:"text"`

	// Readers is a list of sources to read transactions from.
	Readers []string `envconfig:"YNABBER_READERS" default:"nordigen"`

	// Writers is a list of destinations to write transactions to.
	Writers []string `envconfig:"YNABBER_WRITERS" default:"ynab"`

	// Processors is a comma-separated list applied in order before writers.
	// Leave unset or empty to disable processing. Available processors: prefix.
	Processors []string `envconfig:"YNABBER_PROCESSORS"`
}
