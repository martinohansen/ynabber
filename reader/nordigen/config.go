package nordigen

import (
	"fmt"
	"strings"
)

// PayeeSources is a slice of PayeeSource groups which can be a single or
// multiple sources combined. If a group has multiple sources they should be
// combined into a single payee.
type PayeeSources [][]PayeeSource

type PayeeSource uint8

const (
	Name PayeeSource = 1 << iota
	Unstructured
	Additional
)

// Decode value into PayeeSources, each group is separated by a comma and each
// payee in the group is separated by a plus. I.e "name+unstructured,additional"
// will yield two groups: [name, unstructured] and [additional]
func (ps *PayeeSources) Decode(value string) error {
	parts := strings.Split(value, ",")
	for _, part := range parts {
		var group []PayeeSource
		sources := strings.Split(part, "+")
		for _, source := range sources {
			switch strings.TrimSpace(source) {
			case "name":
				group = append(group, Name)
			case "unstructured":
				group = append(group, Unstructured)
			case "additional":
				group = append(group, Additional)
			default:
				return fmt.Errorf("unknown value")
			}
		}
		*ps = append(*ps, group)
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
	PayeeSource PayeeSources `envconfig:"NORDIGEN_PAYEE_SOURCE" default:"unstructured,name,additional"`

	// PayeeStrip is a list of words to remove from Payee. For example:
	// "foo,bar"
	PayeeStrip []string `envconfig:"NORDIGEN_PAYEE_STRIP"`

	// TransactionID is the field to use as transaction ID. Not all banks use
	// the same field and some even change the ID over time.
	//
	// Valid options are: TransactionId, InternalTransactionId
	TransactionID string `envconfig:"NORDIGEN_TRANSACTION_ID" default:"TransactionId"`

	// RequisitionHook is a exec hook thats executed at various stages of the
	// requisition process. The hook is executed with the following arguments:
	// <status> <link>
	RequisitionHook string `envconfig:"NORDIGEN_REQUISITION_HOOK"`

	// RequisitionFile overrides the file used to store the requisition. This
	// file is placed inside the YNABBER_DATADIR.
	RequisitionFile string `envconfig:"NORDIGEN_REQUISITION_FILE"`
}
