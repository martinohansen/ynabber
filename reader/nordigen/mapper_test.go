package nordigen

import (
	"testing"

	"github.com/frieser/nordigen-go-lib/v2"
)

func TestParseDate(t *testing.T) {
	tests := []struct {
		transaction nordigen.Transaction
		wantErr     bool
	}{
		{
			transaction: nordigen.Transaction{
				BookingDate: "2024-01-15",
			},
			wantErr: false,
		},
		{
			transaction: nordigen.Transaction{
				BookingDate: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.transaction.BookingDate, func(t *testing.T) {
			_, err := parseDate(tt.transaction)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
