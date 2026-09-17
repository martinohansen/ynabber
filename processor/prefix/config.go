// Package prefix provides a payee prefix processor for testing pipelines.
package prefix

// Config controls the prefix added to each payee.
type Config struct {
	// Prefix is prepended verbatim, including whitespace, to every payee.
	// Set explicitly to an empty string to leave payees unchanged.
	Prefix string `envconfig:"YNABBER_PREFIX_PAYEE_PREFIX" default:"[test] "`
}
