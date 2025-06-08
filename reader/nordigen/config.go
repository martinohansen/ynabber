// Nordigen reads bank transactions through the Nordigen/GoCardless API. It
// connects to various European banks using PSD2 open banking standards to
// retrieve account information and transaction data.
package nordigen

import (
	"fmt"
	"strings"
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
	// BankID is used to create requisition
	BankID string `envconfig:"NORDIGEN_BANKID"`

	// SecretID is used to create requisition
	SecretID string `envconfig:"NORDIGEN_SECRET_ID"`

	// SecretKey is used to create requisition
	SecretKey string `envconfig:"NORDIGEN_SECRET_KEY"`

	// PayeeSource is a list of sources for Payee candidates, the first method
	// that yields a result will be used. Valid options are: unstructured, name
	// and additional.
	//
	//  * unstructured: uses the `RemittanceInformationUnstructured` field
	//  * name: uses either the either `debtorName` or `creditorName` field
	//  * additional: uses the `AdditionalInformation` field
	//
	// The sources can be combined with the "+" operator. For example:
	// "name+additional,unstructured" will combine name and additional into a
	// single Payee or use unstructured if both are empty.
	PayeeSource PayeeGroups `envconfig:"NORDIGEN_PAYEE_SOURCE" default:"unstructured,name,additional"`

	// PayeeStrip is a list of words to remove from Payee. For example:
	// "foo,bar"
	PayeeStrip []string `envconfig:"NORDIGEN_PAYEE_STRIP"`

	// TransactionID is the field to use as transaction ID. Not all banks use
	// the same field and some even change the ID over time.
	//
	// Valid options are: TransactionId, InternalTransactionId,
	// ProprietaryBankTransactionCode
	TransactionID string `envconfig:"NORDIGEN_TRANSACTION_ID" default:"TransactionId"`

	// RequisitionHook is a exec hook thats executed at various stages of the
	// requisition process. The hook is executed with the following arguments:
	// <status> <link>. Any non-zero exit code will halt the process.
	RequisitionHook string `envconfig:"NORDIGEN_REQUISITION_HOOK"`

	// RequisitionFile overrides the file used to store the requisition. This
	// file is placed inside the YNABBER_DATADIR.
	RequisitionFile string `envconfig:"NORDIGEN_REQUISITION_FILE"`
}
