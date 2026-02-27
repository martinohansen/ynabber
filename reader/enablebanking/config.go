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

// Config holds the configuration for the EnableBanking reader
type Config struct {
	// AppID is the EnableBanking application ID
	AppID string `envconfig:"ENABLEBANKING_APP_ID"`

	// Country is the country code (e.g., NO, SE, DK)
	Country string `envconfig:"ENABLEBANKING_COUNTRY"`

	// ASPSP is the bank identifier (e.g., DNB, Nordea, SparBank)
	ASPSP string `envconfig:"ENABLEBANKING_ASPSP"`

	// RedirectURL is the URL where the user will be redirected after authorization
	RedirectURL string `envconfig:"ENABLEBANKING_REDIRECT_URL"`

	// PEMFile is the path to the private key file for JWT signing
	PEMFile string `envconfig:"ENABLEBANKING_PEM_FILE"`

	// SessionFile is the path where the session is stored for reuse
	SessionFile string `envconfig:"ENABLEBANKING_SESSION_FILE"`

	// FromDate is the start date for transaction retrieval (YYYY-MM-DD format)
	FromDate string `envconfig:"ENABLEBANKING_FROM_DATE"`

	// ToDate is the end date for transaction retrieval (defaults to today)
	ToDate string `envconfig:"ENABLEBANKING_TO_DATE"`

	// Interval is the time between fetches (0 means run once and exit)
	Interval time.Duration `envconfig:"ENABLEBANKING_INTERVAL"`

	// PayeeStrip contains words to remove from payee names.
	// Example: "foo,bar" removes "foo" and "bar" from all payee names.
	PayeeStrip []string `envconfig:"ENABLEBANKING_PAYEE_STRIP"`

	// Debug enables raw JSON dumps of every API response to the local
	// "transactions/" directory for troubleshooting.
	//
	// WARNING: NEVER enable in production. Dumps contain unredacted account
	// details, session tokens, and full transaction history in plain text.
	Debug bool `envconfig:"ENABLEBANKING_DEBUG"`
}

// Validate checks that required fields are set and sets defaults.
// dataDir is the base directory for the session file (from YNABBER_DATADIR).
func (c *Config) Validate(dataDir string) error {
	if c.AppID == "" {
		return fmt.Errorf("ENABLEBANKING_APP_ID is required")
	}
	if c.Country == "" {
		return fmt.Errorf("ENABLEBANKING_COUNTRY is required")
	}
	if c.ASPSP == "" {
		return fmt.Errorf("ENABLEBANKING_ASPSP is required")
	}
	if c.RedirectURL == "" {
		return fmt.Errorf("ENABLEBANKING_REDIRECT_URL is required")
	}
	if c.PEMFile == "" {
		return fmt.Errorf("ENABLEBANKING_PEM_FILE is required")
	}
	if c.FromDate == "" {
		return fmt.Errorf("ENABLEBANKING_FROM_DATE is required")
	}

	// Validate FromDate format
	if _, err := time.Parse("2006-01-02", c.FromDate); err != nil {
		return fmt.Errorf("invalid ENABLEBANKING_FROM_DATE format (expected YYYY-MM-DD): %w", err)
	}

	// Set ToDate to today if not provided
	if c.ToDate == "" {
		c.ToDate = time.Now().Format("2006-01-02")
	} else {
		// Validate ToDate format if provided
		if _, err := time.Parse("2006-01-02", c.ToDate); err != nil {
			return fmt.Errorf("invalid ENABLEBANKING_TO_DATE format (expected YYYY-MM-DD): %w", err)
		}
	}

	// Set default session file if not provided
	if c.SessionFile == "" {
		c.SessionFile = filepath.Join(dataDir, defaultSessionFile(c.ASPSP, c.Country))
	}

	return nil
}

// GetFromDate parses and returns the FromDate as a time.Time
func (c Config) GetFromDate() (time.Time, error) {
	return time.Parse("2006-01-02", c.FromDate)
}

// GetToDate parses and returns the ToDate as a time.Time
func (c Config) GetToDate() (time.Time, error) {
	if c.ToDate == "" {
		return time.Now(), nil
	}
	return time.Parse("2006-01-02", c.ToDate)
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
