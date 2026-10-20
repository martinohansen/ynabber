// EnableBanking reads bank transactions through the EnableBanking Open Banking API.
// It connects to various European banks using PSD2 open banking standards to retrieve
// account information and transaction data.
package enablebanking

import (
	"fmt"
	"strings"
	"time"
)

const dateFormat = "2006-01-02"

// PSU types accepted by the EnableBanking authorization endpoint. Which ones a
// given bank supports is listed as psu_types in GET /aspsps.
const (
	psuTypePersonal = "personal"
	psuTypeBusiness = "business"
)

// PSUType is the payment service user type used for bank consent.
type PSUType string

// Decode parses the payment service user type used for bank consent.
func (p *PSUType) Decode(value string) error {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "":
		*p = PSUType(psuTypePersonal)
	case psuTypePersonal, psuTypeBusiness:
		*p = PSUType(normalized)
	default:
		return fmt.Errorf(
			"invalid PSU type %q: must be %q or %q",
			value, psuTypePersonal, psuTypeBusiness,
		)
	}
	return nil
}

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

	// ToDate is the end date for transaction retrieval.
	// When omitted, it resolves dynamically to the current UTC date on each run.
	ToDate Date `envconfig:"ENABLEBANKING_TO_DATE"`

	// Interval is the time between fetches (0 means run once and exit)
	Interval time.Duration `envconfig:"ENABLEBANKING_INTERVAL"`

	// PayeeStrip contains words to remove from payee names.
	// Example: "foo,bar" removes "foo" and "bar" from all payee names.
	PayeeStrip []string `envconfig:"ENABLEBANKING_PAYEE_STRIP"`

	// PayeeStripRegex is a comma-separated list of regular expressions whose
	// matches are removed from payee names. Use it to strip dynamic prefixes
	// or codes that PayeeStrip can't express. Patterns cannot contain a
	// literal comma.
	// Example: "^Dk-Nota\S+\s+" turns "Dk-Nota61221 Remouladen" into
	// "Remouladen".
	PayeeStripRegex PayeeRegex `envconfig:"ENABLEBANKING_PAYEE_STRIP_REGEX"`

	// PSUHeaders controls whether PSU headers are sent to EnableBanking.
	// Leave it unset to enable them only for banks that require them, such as
	// Bulder and Sparebanken Vest. Set it to true to always enable the headers,
	// or false to always disable them.
	PSUHeaders *bool `envconfig:"ENABLEBANKING_PSU_HEADERS"`

	// PSUIPAddress is an optional end-user IP address sent to EnableBanking.
	// The value is sent as configured. When PSU headers are enabled, Ynabber
	// discovers the public IP address if this value is empty.
	PSUIPAddress string `envconfig:"ENABLEBANKING_PSU_IP_ADDRESS"`

	// PSUUserAgent is the User-Agent value sent in the PSU-User-Agent header.
	PSUUserAgent string `envconfig:"ENABLEBANKING_PSU_USER_AGENT" default:"Mozilla/5.0 (compatible; Ynabber/1.0)"`

	// PSUType is the payment service user type requested when creating a
	// session: "personal" or "business". Banks expose company accounts only
	// under "business", so a personal consent returns the private accounts
	// even for a user who also signs for a company. GET /aspsps lists which
	// types each bank supports as psu_types.
	//
	// Changing this for an existing connection requires deleting the session
	// file first, because the stored session holds the accounts granted by
	// the previous consent.
	PSUType PSUType `envconfig:"ENABLEBANKING_PSU_TYPE" default:"personal"`
}

// psuTypeOrDefault returns the configured PSU type, falling back to "personal"
// for a Config constructed directly in Go.
func (c Config) psuTypeOrDefault() string {
	if c.PSUType == "" {
		return psuTypePersonal
	}
	return string(c.PSUType)
}

// GetFromDate returns FromDate as a time.Time. Environment loading parses and validates the date.
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
