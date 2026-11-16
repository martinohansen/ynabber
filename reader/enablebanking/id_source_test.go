package enablebanking

import "testing"

func TestTransactionIDSource(t *testing.T) {
	tests := []struct {
		name string
		tx   EBTransaction
		want string
	}{
		{"entry reference wins", EBTransaction{EntryReference: "E1", ReferenceNumber: "R1"}, "entry_reference"},
		{"reference number", EBTransaction{ReferenceNumber: "R1"}, "reference_number"},
		{"synthetic fallback", EBTransaction{RemittanceInformation: []string{"Visa, Kiwi"}}, "synthetic"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := transactionIDSource(tt.tx); got != tt.want {
				t.Fatalf("transactionIDSource() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRemittanceShape(t *testing.T) {
	tests := []struct {
		name string
		rem  []string
		want string
	}{
		{"empty", nil, "empty/0"},
		{"digits first", []string{"5909", "Giro, Vibb As, Efaktura"}, "digits-first/2"},
		{"text first", []string{"Giro 5909, Vibb As, Efaktura"}, "text-first/1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := remittanceShape(EBTransaction{RemittanceInformation: tt.rem}); got != tt.want {
				t.Fatalf("remittanceShape() = %q, want %q", got, tt.want)
			}
		})
	}
}
