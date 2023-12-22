package json

import (
	"encoding/json"
	"fmt"

	"github.com/martinohansen/ynabber"
)

type Writer struct{}

func (w Writer) Bulk(tx []ynabber.Transaction) error {
	b, err := json.MarshalIndent(tx, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling: %w", err)
	}
	fmt.Println(string(b))
	return nil
}
