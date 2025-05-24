package nordigen

import (
	"strings"

	"github.com/frieser/nordigen-go-lib/v2"
	"github.com/martinohansen/ynabber"
)

// nordeaMapper handles Nordea transactions specifically
func (r Reader) nordeaMapper(a ynabber.Account, t nordigen.Transaction) (*ynabber.Transaction, error) {
	// They now maintain two transactions for every actual transaction. First
	// they show up prefixed with a ID prefixed with a H, sometime later another
	// transaction describing the same transactions shows up with a new ID
	// prefixed with a P instead. The H transaction matches the date which its
	// visible in my account so i will discard the P transactions for now.
	if strings.HasPrefix(t.TransactionId, "P") {
		return nil, nil
	}

	return r.defaultMapper(a, t)
}
