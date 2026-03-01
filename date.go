package ynabber

import (
	"fmt"
	"strings"
	"time"
)

// DateFormat is the canonical date format used throughout ynabber: YYYY-MM-DD.
const DateFormat = "2006-01-02"

// Date is a time.Time that knows how to decode itself from a YYYY-MM-DD string.
// It implements envconfig.Decoder so that any Config struct field typed as Date
// is parsed exactly once at load time, eliminating manual time.Parse calls and
// repeated re-parsing in accessor methods.
//
// Dates are always stored in UTC. MarshalJSON and UnmarshalJSON are implemented
// so that JSON serialisation round-trips correctly as "YYYY-MM-DD" rather than
// silently producing an empty object.
type Date time.Time

// Decode implements envconfig.Decoder, parsing a YYYY-MM-DD string into Date.
func (d *Date) Decode(value string) error {
	t, err := time.Parse(DateFormat, value)
	if err != nil {
		return err
	}
	*d = Date(t)
	return nil
}

// MarshalJSON encodes the date as a JSON string in YYYY-MM-DD format (UTC).
func (d Date) MarshalJSON() ([]byte, error) {
	return []byte(`"` + time.Time(d).UTC().Format(DateFormat) + `"`), nil
}

// UnmarshalJSON decodes a JSON string in YYYY-MM-DD format into Date.
func (d *Date) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	if s == "null" {
		return nil
	}
	t, err := time.Parse(DateFormat, s)
	if err != nil {
		return fmt.Errorf("ynabber.Date: cannot unmarshal %q: %w", s, err)
	}
	*d = Date(t)
	return nil
}
