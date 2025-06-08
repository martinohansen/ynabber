// Nordigen reads bank transactions through the Nordigen/GoCardless API. It
// connects to various European banks using PSD2 open banking standards to
// retrieve account information and transaction data.
package nordigen

import (
	"fmt"
	"strings"
	"time"
)

// PayeeGroups is a slice of PayeeSources which can be one or more sources. If a
// group has multiple sources they should be combined into a single payee.
type PayeeGroups [][]PayeeSource

type PayeeSource uint8

const (
	Name PayeeSource = 1 << iota
	Unstructured
	Additional
)

// Decode value into PayeeSources, each group is separated by a comma and each
// payee in the group is separated by a plus. I.e "name+unstructured,additional"
// will yield two groups: [name, unstructured] and [additional]
func (pg *PayeeGroups) Decode(value string) error {
	groups := strings.Split(value, ",")
	for _, group := range groups {
		var sources []PayeeSource
		for _, source := range strings.Split(group, "+") {
			switch strings.TrimSpace(source) {
			case "name":
				sources = append(sources, Name)
			case "unstructured":
				sources = append(sources, Unstructured)
			case "additional":
				sources = append(sources, Additional)
			default:
				return fmt.Errorf("unknown value")
			}
		}
		*pg = append(*pg, sources)
	}
	return nil
}

type Config struct {
	// BankID identifies the bank for creating requisitions
	BankID string `envconfig:"NORDIGEN_BANKID"`

	// SecretID is the client ID for API authentication
	SecretID string `envconfig:"NORDIGEN_SECRET_ID"`

	// SecretKey is the client secret for API authentication
	SecretKey string `envconfig:"NORDIGEN_SECRET_KEY"`

	// PayeeSource defines the sources and order for extracting payee
	// information. Multiple sources can be combined with "+" to merge their
	// values. Groups are separated by "," and tried in order until a non-empty
	// result is found.
	//
	// Available sources:
	//  * unstructured: uses the RemittanceInformationUnstructured field
	//  * name: uses either the debtorName or creditorName field
	//  * additional: uses the AdditionalInformation field
	//
	// Example: "name+additional,unstructured" will first try to combine name
	// and additional fields, falling back to unstructured if both are empty.
	PayeeSource PayeeGroups `envconfig:"NORDIGEN_PAYEE_SOURCE" default:"unstructured,name,additional"`

	// PayeeStrip contains words to remove from payee names.
	// Example: "foo,bar" removes "foo" and "bar" from all payee names.
	PayeeStrip []string `envconfig:"NORDIGEN_PAYEE_STRIP"`

	// TransactionID specifies which field to use as the unique transaction
	// identifier. Banks may use different fields, and some change the ID format
	// over time.
	//
	// Valid options: TransactionId, InternalTransactionId,
	// ProprietaryBankTransactionCode
	TransactionID string `envconfig:"NORDIGEN_TRANSACTION_ID" default:"TransactionId"`

	// RequisitionHook is an executable that runs at various stages of the
	// requisition process. It receives arguments: <status> <link>
	// Non-zero exit codes will stop the process.
	RequisitionHook string `envconfig:"NORDIGEN_REQUISITION_HOOK"`

	// RequisitionFile specifies the filename for storing requisition data.
	// The file is stored in the directory defined by YNABBER_DATADIR.
	RequisitionFile string `envconfig:"NORDIGEN_REQUISITION_FILE"`

	// Interval determines how often to fetch new transactions.
	// Set to 0 to run only once instead of continuously.
	Interval time.Duration `envconfig:"NORDIGEN_INTERVAL" default:"6h"`
}
