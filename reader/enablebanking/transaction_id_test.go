package enablebanking

import (
	"log/slog"
	"os"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// resolveTransactionID
//
// These tests encode the NEW contract defined in the architecture for #154:
//   Priority: entry_reference → reference_number → syntheticTransactionID
//   transaction_id is intentionally excluded from the chain.
//
// All tests below will FAIL (or not compile) against the current implementation
// because:
//  1. resolveTransactionID currently returns (string, error); the new signature
//     returns only string.
//  2. syntheticTransactionID does not exist yet.
// ---------------------------------------------------------------------------

// TestResolveTransactionID_Priority verifies the full resolution priority chain
// using real field combinations observed in the three EnableBanking ASPSPs.
func TestResolveTransactionID_Priority(t *testing.T) {
	tests := []struct {
		name         string
		tx           EBTransaction
		want         string // exact match when wantSynth is false
		wantSynth    bool   // true → only assert "synth:" prefix
		wantNotEqual string // if non-empty, result must NOT equal this value
	}{
		{
			// Sparebanken: entry_reference is stable; transaction_id is null.
			// Must use entry_reference.
			name: "sparebanken: entry_reference used when transaction_id is null",
			tx: EBTransaction{
				EntryReference: "2026-02-25-0",
				TransactionID:  "",
				BookingDate:    "2026-02-25",
				TransactionAmount: struct {
					Currency string `json:"currency"`
					Amount   string `json:"amount"`
				}{Currency: "NOK", Amount: "29046.00"},
				CreditDebitIndicator: "DBIT",
			},
			want: "2026-02-25-0",
		},
		{
			// Credit card: both entry_reference and transaction_id are present.
			// entry_reference must win — transaction_id is just a base64
			// re-encoding of the same value and must not be preferred.
			name: "credit card: entry_reference preferred over transaction_id",
			tx: EBTransaction{
				EntryReference: "202603020208822",
				TransactionID:  "MjAyNjAzMDIwMjA4ODIy", // base64("202603020208822")
				BookingDate:    "2026-03-02",
				TransactionAmount: struct {
					Currency string `json:"currency"`
					Amount   string `json:"amount"`
				}{Currency: "NOK", Amount: "15237.0"},
				CreditDebitIndicator:  "CRDT",
				RemittanceInformation: []string{"INNBETALING BANKGIRO"},
			},
			want: "202603020208822",
		},
		{
			// DNB: entry_reference is null, transaction_id is session-scoped
			// (base64 of a counter-prefixed string that changes on re-auth).
			// Must return a synthetic ID, never the session-scoped transaction_id.
			name: "dnb: entry_reference null → synthetic, NOT transaction_id",
			tx: EBTransaction{
				EntryReference: nil,
				TransactionID:  "SESSIONTOKEN_SYNTHETIC_TESTONLY",
				BookingDate:    "2000-01-01",
				TransactionAmount: struct {
					Currency string `json:"currency"`
					Amount   string `json:"amount"`
				}{Currency: "NOK", Amount: "100.00"},
				CreditDebitIndicator:  "DBIT",
				RemittanceInformation: []string{"Test remittance synthetic data"},
			},
			wantSynth:    true,
			wantNotEqual: "SESSIONTOKEN_SYNTHETIC_TESTONLY",
		},
		{
			// reference_number fallback: entry_reference null, reference_number
			// present. Must use reference_number before falling to synthetic.
			name: "reference_number used when entry_reference is null",
			tx: EBTransaction{
				EntryReference:  nil,
				ReferenceNumber: "REF-STABLE-123",
				TransactionID:   "session-scoped-id",
				BookingDate:     "2026-01-10",
				TransactionAmount: struct {
					Currency string `json:"currency"`
					Amount   string `json:"amount"`
				}{Currency: "NOK", Amount: "500.00"},
				CreditDebitIndicator: "DBIT",
			},
			want: "REF-STABLE-123",
		},
		{
			// All ID fields null/empty: must return synthetic, never an error.
			// This is the core regression case for issue #154.
			name: "all id fields null → synthetic returned (no error)",
			tx: EBTransaction{
				EntryReference:  nil,
				ReferenceNumber: nil,
				TransactionID:   "",
				BookingDate:     "2026-02-27",
				TransactionAmount: struct {
					Currency string `json:"currency"`
					Amount   string `json:"amount"`
				}{Currency: "NOK", Amount: "9237.33"},
				CreditDebitIndicator:  "DBIT",
				RemittanceInformation: []string{"Lån, Lån 9802.55.21212"},
			},
			wantSynth: true,
		},
		{
			// entry_reference with leading/trailing whitespace must be trimmed
			// and still match.
			name: "entry_reference with whitespace is trimmed",
			tx: EBTransaction{
				EntryReference: "  2026-02-25-0  ",
				TransactionID:  "ignored",
			},
			want: "2026-02-25-0",
		},
		{
			// entry_reference set to the JSON null literal (interface{} nil).
			// Must fall through to synthetic, not use transaction_id.
			name: "entry_reference interface nil treated as absent",
			tx: EBTransaction{
				EntryReference:  nil,
				ReferenceNumber: nil,
				TransactionID:   "should-be-ignored",
				BookingDate:     "2026-01-01",
				TransactionAmount: struct {
					Currency string `json:"currency"`
					Amount   string `json:"amount"`
				}{Currency: "NOK", Amount: "1.00"},
				CreditDebitIndicator: "CRDT",
			},
			wantSynth:    true,
			wantNotEqual: "should-be-ignored",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// NOTE: resolveTransactionID is called with the NEW signature
			// (returns string, not (string, error)). This will not compile
			// against the current implementation — that is intentional (Red).
			got := resolveTransactionID(tt.tx)

			if got == "" {
				t.Fatalf("resolveTransactionID() returned empty string")
			}

			if tt.wantSynth {
				if !strings.HasPrefix(got, "synth:") {
					t.Errorf("resolveTransactionID() = %q, want prefix %q", got, "synth:")
				}
			} else {
				if got != tt.want {
					t.Errorf("resolveTransactionID() = %q, want %q", got, tt.want)
				}
			}

			if tt.wantNotEqual != "" && got == tt.wantNotEqual {
				t.Errorf("resolveTransactionID() = %q, must NOT equal %q (session-scoped value)", got, tt.wantNotEqual)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// syntheticTransactionID
// ---------------------------------------------------------------------------

// TestSyntheticTransactionID verifies the properties of the synthetic ID
// generator: stable prefix, determinism, and sensitivity to each input field.
//
// NOTE: syntheticTransactionID does not exist yet — this file will not compile
// until the Dev step adds the function.
func TestSyntheticTransactionID(t *testing.T) {
	base := EBTransaction{
		BookingDate: "2000-01-01",
		TransactionAmount: struct {
			Currency string `json:"currency"`
			Amount   string `json:"amount"`
		}{Currency: "NOK", Amount: "100.00"},
		CreditDebitIndicator:  "DBIT",
		RemittanceInformation: []string{"Test remittance synthetic data"},
	}

	t.Run("starts with synth: prefix", func(t *testing.T) {
		got := syntheticTransactionID(base)
		if !strings.HasPrefix(got, "synth:") {
			t.Errorf("syntheticTransactionID() = %q, want prefix %q", got, "synth:")
		}
	})

	t.Run("deterministic: identical inputs produce identical output", func(t *testing.T) {
		first := syntheticTransactionID(base)
		second := syntheticTransactionID(base)
		if first != second {
			t.Errorf("non-deterministic: first call = %q, second call = %q", first, second)
		}
	})

	t.Run("booking_date change produces different id", func(t *testing.T) {
		other := base
		other.BookingDate = "2026-02-28"
		if syntheticTransactionID(base) == syntheticTransactionID(other) {
			t.Error("different booking_date must produce different synthetic ID")
		}
	})

	t.Run("amount change produces different id", func(t *testing.T) {
		other := base
		other.TransactionAmount.Amount = "9237.34"
		if syntheticTransactionID(base) == syntheticTransactionID(other) {
			t.Error("different amount must produce different synthetic ID")
		}
	})

	t.Run("currency change produces different id", func(t *testing.T) {
		other := base
		other.TransactionAmount.Currency = "EUR"
		if syntheticTransactionID(base) == syntheticTransactionID(other) {
			t.Error("different currency must produce different synthetic ID")
		}
	})

	t.Run("credit_debit_indicator change produces different id", func(t *testing.T) {
		other := base
		other.CreditDebitIndicator = "CRDT"
		if syntheticTransactionID(base) == syntheticTransactionID(other) {
			t.Error("different credit_debit_indicator must produce different synthetic ID")
		}
	})

	t.Run("remittance_information change produces different id", func(t *testing.T) {
		other := base
		other.RemittanceInformation = []string{"completely different remittance text"}
		if syntheticTransactionID(base) == syntheticTransactionID(other) {
			t.Error("different remittance_information must produce different synthetic ID")
		}
	})

	t.Run("empty remittance_information does not panic", func(t *testing.T) {
		empty := base
		empty.RemittanceInformation = []string{}
		got := syntheticTransactionID(empty)
		if got == "" {
			t.Error("empty remittance_information must still produce a non-empty ID")
		}
		if !strings.HasPrefix(got, "synth:") {
			t.Errorf("empty remittance: got %q, want synth: prefix", got)
		}
	})

	t.Run("remittance pipe in entry does not cause intra-list collision", func(t *testing.T) {
		// ["a|b", "c"] and ["a", "b|c"] must hash differently.
		// JSON encoding preserves element boundaries, preventing this collision.
		txA := base
		txA.RemittanceInformation = []string{"a|b", "c"}
		txB := base
		txB.RemittanceInformation = []string{"a", "b|c"}
		if syntheticTransactionID(txA) == syntheticTransactionID(txB) {
			t.Error("pipe in remittance entry must not cause intra-list collision")
		}
	})

	t.Run("nil remittance_information does not panic", func(t *testing.T) {
		nilRemit := base
		nilRemit.RemittanceInformation = nil
		got := syntheticTransactionID(nilRemit)
		if got == "" {
			t.Error("nil remittance_information must still produce a non-empty ID")
		}
	})

	t.Run("field boundary: adjacent-field collision resistance", func(t *testing.T) {
		// "abc" + "def" must not collide with "ab" + "cdef".
		// Achieved by JSON encoding, which quotes each field individually.
		txA := EBTransaction{
			BookingDate: "2026-01-01",
			TransactionAmount: struct {
				Currency string `json:"currency"`
				Amount   string `json:"amount"`
			}{Currency: "NOK", Amount: "100"},
			CreditDebitIndicator:  "DBIT",
			RemittanceInformation: []string{"x"},
		}
		txB := EBTransaction{
			BookingDate: "2026-01-0", // one char shorter in date
			TransactionAmount: struct {
				Currency string `json:"currency"`
				Amount   string `json:"amount"`
			}{Currency: "1NOK", Amount: "00"}, // currency absorbs the moved char
			CreditDebitIndicator:  "DBIT",
			RemittanceInformation: []string{"x"},
		}
		if syntheticTransactionID(txA) == syntheticTransactionID(txB) {
			t.Error("JSON encoding must prevent cross-field collision")
		}
	})
}

// ---------------------------------------------------------------------------
// Integration: Mapper with DNB-like null entry_reference
// ---------------------------------------------------------------------------

// TestMapperDNBNullEntryReference is an end-to-end test using the real DNB
// example payload from env/example_transactions.md.
//
// Assertions:
//  1. Mapper does not return an error.
//  2. The resulting transaction ID starts with "synth:".
//  3. Calling Mapper a second time with the same data produces the same ID
//     (session-independence / determinism property).
func TestMapperDNBNullEntryReference(t *testing.T) {
	reader := Reader{
		logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	account := AccountInfo{
		UID:         "dnb-acc-uid",
		AccountID:   AccountID{IBAN: randomTestIBAN(t)},
		DisplayName: "DNB Account",
	}

	// Synthetic DNB-like payload — mirrors the shape of a real DNB transaction
	// (entry_reference null, session-scoped transaction_id) without using any
	// real account numbers, loan references, or transaction identifiers.
	dnbTx := EBTransaction{
		EntryReference: nil,
		TransactionID:  "SESSIONTOKEN_SYNTHETIC_TESTONLY",
		TransactionAmount: struct {
			Currency string `json:"currency"`
			Amount   string `json:"amount"`
		}{Currency: "NOK", Amount: "100.00"},
		CreditDebitIndicator:  "DBIT",
		Status:                "BOOK",
		BookingDate:           "2000-01-01",
		ValueDate:             "2000-01-01",
		RemittanceInformation: []string{"Test remittance synthetic data"},
	}

	tx1, err := reader.Mapper(account, dnbTx)
	if err != nil {
		t.Fatalf("Mapper() returned unexpected error: %v", err)
	}
	if tx1 == nil {
		t.Fatal("Mapper() returned nil transaction")
	}

	if !strings.HasPrefix(string(tx1.ID), "synth:") {
		t.Errorf("tx.ID = %q, want prefix %q — DNB transaction must use synthetic ID", tx1.ID, "synth:")
	}

	// Call again with identical data: ID must be stable (session-independent).
	tx2, err := reader.Mapper(account, dnbTx)
	if err != nil {
		t.Fatalf("second Mapper() call returned unexpected error: %v", err)
	}
	if tx1.ID != tx2.ID {
		t.Errorf("non-deterministic ID: first=%q second=%q — ID must be stable across calls", tx1.ID, tx2.ID)
	}
}

// TestMapperAllIDFieldsNull verifies that a transaction where every ID field
// is null/empty is still successfully mapped (no error) and receives a synthetic
// ID. This is the core regression guard for issue #154.
func TestMapperAllIDFieldsNull(t *testing.T) {
	reader := Reader{
		logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	account := AccountInfo{
		UID:       "acc-null-ids",
		AccountID: AccountID{IBAN: randomTestIBAN(t)},
	}

	tx := EBTransaction{
		EntryReference:  nil,
		ReferenceNumber: nil,
		TransactionID:   "",
		BookingDate:     "2026-01-15",
		TransactionAmount: struct {
			Currency string `json:"currency"`
			Amount   string `json:"amount"`
		}{Currency: "NOK", Amount: "42.00"},
		CreditDebitIndicator:  "CRDT",
		Status:                "BOOK",
		RemittanceInformation: []string{"Some payment"},
	}

	result, err := reader.Mapper(account, tx)
	if err != nil {
		t.Fatalf("Mapper() returned error for all-null ID fields: %v\n"+
			"  After fix, a synthetic ID must be generated instead of returning an error.", err)
	}
	if result == nil {
		t.Fatal("Mapper() returned nil")
	}
	if !strings.HasPrefix(string(result.ID), "synth:") {
		t.Errorf("result.ID = %q, want synth: prefix", result.ID)
	}
}
