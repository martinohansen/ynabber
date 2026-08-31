// Wealthreader reads bank transactions through the Wealth Reader AIS API
// (https://www.wealthreader.com/docs/en/). The self-hoster brings their own
// api_key (BYO). First run does an OAuth redirect; later runs refresh with
// POST /entities/ using the stored token + institution code.
package wealthreader

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

// Config holds the configuration for the Wealthreader reader.
//
// Wealth Reader contract this maps to:
//   - ApiKey: BYO key from the client area (POST /entities/ and /domains/).
//   - Code: institution code (bbva, caixabank, …) from GET /entities/.
//   - RedirectURL: must be registered with access_type=oauth and tokenize=1.
//   - ApiBase / OAuthBase: override for sandbox; production defaults below.
type Config struct {
	// ApiKey is the Wealth Reader API key (client area / onboarding).
	ApiKey string `envconfig:"WEALTHREADER_API_KEY" required:"true"`

	// Code is the institution code (e.g. bbva, caixabank). Same value as
	// statistics.code and the `code` field of POST /entities/.
	Code string `envconfig:"WEALTHREADER_CODE" required:"true"`

	// RedirectURL is the OAuth return URL. It must match the domain registered
	// with method=add&access_type=oauth on https://api.wealthreader.com/domains/.
	RedirectURL string `envconfig:"WEALTHREADER_REDIRECT_URL" required:"true"`

	// SessionFile is the path where the token + institution code are stored.
	SessionFile string `envconfig:"WEALTHREADER_SESSION_FILE"`

	// FromDate is the start date for transaction retrieval (YYYY-MM-DD).
	// Sent as date_from on POST /entities/. Wealth Reader defaults to yesterday
	// when the field is omitted; we always send it so the window is explicit.
	FromDate Date `envconfig:"WEALTHREADER_FROM_DATE" required:"true"`

	// ToDate is the end date for transaction retrieval.
	// When omitted, it resolves dynamically to the current UTC date on each run.
	ToDate Date `envconfig:"WEALTHREADER_TO_DATE"`

	// Interval is the time between fetches (0 means run once and exit).
	Interval time.Duration `envconfig:"WEALTHREADER_INTERVAL"`

	// ProductTypes is the comma-separated product_types sent on refresh
	// (accounts, portfolios, cards, …). Default is accounts-only.
	ProductTypes string `envconfig:"WEALTHREADER_PRODUCT_TYPES" default:"accounts"`

	// ApiBase overrides https://api.wealthreader.com (dev/mock).
	ApiBase string `envconfig:"WEALTHREADER_API_BASE" default:"https://api.wealthreader.com"`

	// OAuthBase overrides https://oauth.wealthreader.com (dev/mock).
	OAuthBase string `envconfig:"WEALTHREADER_OAUTH_BASE" default:"https://oauth.wealthreader.com"`

	// PayeeStrip contains words to remove from payee names.
	PayeeStrip []string `envconfig:"WEALTHREADER_PAYEE_STRIP"`

	// PayeeStripRegex is a comma-separated list of regular expressions whose
	// matches are removed from payee names. Patterns cannot contain a comma.
	PayeeStripRegex PayeeRegex `envconfig:"WEALTHREADER_PAYEE_STRIP_REGEX"`
}

// Validate checks config semantics and sets defaults for optional fields.
// dataDir is the base directory for the session file (from YNABBER_DATADIR).
func (c *Config) Validate(dataDir string) error {
	if c.SessionFile == "" {
		c.SessionFile = filepath.Join(dataDir, defaultSessionFile(c.Code))
	}
	c.ApiBase = strings.TrimRight(c.ApiBase, "/")
	c.OAuthBase = strings.TrimRight(c.OAuthBase, "/")
	return nil
}

// GetFromDate returns FromDate as a time.Time. It is always valid after Validate.
func (c Config) GetFromDate() (time.Time, error) {
	return time.Time(c.FromDate), nil
}

// GetToDate returns ToDate as a time.Time.
// When ToDate is omitted, it resolves to the current UTC time so repeated runs
// continue to advance the effective date window.
func (c Config) GetToDate() (time.Time, error) {
	if time.Time(c.ToDate).IsZero() {
		return time.Now().UTC(), nil
	}
	return time.Time(c.ToDate), nil
}

func defaultSessionFile(code string) string {
	return fmt.Sprintf("wealthreader_%s_session.json", sanitizeSessionPart(code))
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
