package nordigen

import (
	"fmt"
	"testing"

	"github.com/martinohansen/nordigen-go-lib/v2"
)

func TestParseAmount(t *testing.T) {
	tests := []struct {
		transaction nordigen.Transaction
		want        float64
		wantErr     bool
	}{
		{
			transaction: nordigen.Transaction{
				TransactionAmount: struct {
					Amount   string "json:\"amount,omitempty\""
					Currency string "json:\"currency,omitempty\""
				}{Amount: "328.18"},
			},
			want:    328.18,
			wantErr: false,
		},
		{
			transaction: nordigen.Transaction{
				TransactionAmount: struct {
					Amount   string "json:\"amount,omitempty\""
					Currency string "json:\"currency,omitempty\""
				}{Amount: "32818"},
			},
			want:    32818,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Amount: %s", tt.transaction.TransactionAmount.Amount), func(t *testing.T) {
			got, err := parseAmount(tt.transaction)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("got = %v, want %v", got, tt.want)
			}
		})
	}
}
