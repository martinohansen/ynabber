// EnableBanking reads bank transactions through the EnableBanking Open Banking API.
// It connects to various European banks using PSD2 open banking standards to retrieve
// account information and transaction data.
package enablebanking

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

const dateFormat = "2006-01-02"

type Date time.Time

// Decode implements envconfig.Decoder, parsing a YYYY-MM-DD string into Date.
func (d *Date) Decode(value string) error {
	t, err := time.Parse(dateFormat, value)
	if err != nil {
		return err
	}
	*d = Date(t)
	return nil
}

// Config holds the configuration for the EnableBanking reader
type Config struct {
	// AppID is the EnableBanking application ID
	AppID string `envconfig:"ENABLEBANKING_APP_ID" required:"true"`

	// Country is the country code (e.g., NO, SE, DK)
	Country string `envconfig:"ENABLEBANKING_COUNTRY" required:"true"`

	// ASPSP is the bank identifier (e.g., DNB, Nordea, SparBank)
	ASPSP string `envconfig:"ENABLEBANKING_ASPSP" required:"true"`

	// RedirectURL is the URL where the user will be redirected after authorization
	RedirectURL string `envconfig:"ENABLEBANKING_REDIRECT_URL" default:"https://martinohansen.github.io/ynabber/ok.html"`

	// PEMFile is the path to the private key file for JWT signing
	PEMFile string `envconfig:"ENABLEBANKING_PEM_FILE" required:"true"`

	// SessionFile is the path where the session is stored for reuse
	SessionFile string `envconfig:"ENABLEBANKING_SESSION_FILE"`

	// FromDate is the start date for transaction retrieval (YYYY-MM-DD format).
	FromDate Date `envconfig:"ENABLEBANKING_FROM_DATE" required:"true"`

	// ToDate is the end date for transaction retrieval (defaults to today).
	ToDate Date `envconfig:"ENABLEBANKING_TO_DATE"`

	// Interval is the time between fetches (0 means run once and exit)
	Interval time.Duration `envconfig:"ENABLEBANKING_INTERVAL"`

	// PayeeStrip contains words to remove from payee names.
	// Example: "foo,bar" removes "foo" and "bar" from all payee names.
	PayeeStrip []string `envconfig:"ENABLEBANKING_PAYEE_STRIP"`
}

// Validate checks config semantics and sets defaults for optional fields.
// dataDir is the base directory for the session file (from YNABBER_DATADIR).
func (c *Config) Validate(dataDir string) error {
	// Default ToDate to today if not provided.
	if time.Time(c.ToDate).IsZero() {
		c.ToDate = Date(time.Now().UTC())
	}

	// Set default session file if not provided
	if c.SessionFile == "" {
		c.SessionFile = filepath.Join(dataDir, defaultSessionFile(c.ASPSP, c.Country))
	}

	return nil
}

// GetFromDate returns FromDate as a time.Time. It is always valid after Validate.
func (c Config) GetFromDate() (time.Time, error) {
	return time.Time(c.FromDate), nil
}

// GetToDate returns ToDate as a time.Time. It is always valid after Validate.
func (c Config) GetToDate() (time.Time, error) {
	return time.Time(c.ToDate), nil
}

func defaultSessionFile(aspsp, country string) string {
	return fmt.Sprintf("enablebanking_%s_%s_session.json", sanitizeSessionPart(aspsp), sanitizeSessionPart(country))
}

func sanitizeSessionPart(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	trimmed = strings.ReplaceAll(trimmed, " ", "_")
	trimmed = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '_' || r == '-':
			return r
		default:
			return -1
		}
	}, trimmed)
	if trimmed == "" {
		return "unknown"
	}
	return trimmed
}
