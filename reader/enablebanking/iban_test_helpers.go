package enablebanking

import (
	"crypto/rand"
	"testing"
)

func randomTestIBAN(t *testing.T) string {
	t.Helper()

	const countryCode = "NO"
	const digits = 13
	buf := make([]byte, digits)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("failed to generate test IBAN: %v", err)
	}
	for i, b := range buf {
		buf[i] = '0' + (b % 10)
	}
	return countryCode + string(buf)
}
