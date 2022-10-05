package nordigen

import (
	"encoding/json"
	"testing"

	"github.com/frieser/nordigen-go-lib/v2"
	"github.com/martinohansen/ynabber"
)

// getTransactions takes json with input and returns parsed transactions
func getTransactions(t *testing.T, input string) []ynabber.Transaction {
	var dummy_transactions nordigen.AccountTransactions
	json.Unmarshal([]byte(input), &dummy_transactions)

	parsed, err := transactionsToYnabber(ynabber.Account{}, dummy_transactions)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func TestTransactionsToYnabber(t *testing.T) {
	transactions := getTransactions(t, `{
    "transactions": {
        "booked": [
            {
                "transactionAmount": {
                    "amount": "328.18"
                },
                "bookingDate": "2006-01-02"
            },
            {
                "transactionAmount": {
                    "amount": "32818"
                },
                "bookingDate": "2006-01-02"
            }
        ]
    }
}
`)

	want := ynabber.Milliunits(328180)
	got := transactions[0].Amount
	if got != want {
		t.Fatalf("failed to parse amount: %s != %s", got, want)
	}

	want = ynabber.Milliunits(32818000)
	got = transactions[1].Amount
	if got != want {
		t.Fatalf("failed to parse amount: %s != %s", got, want)
	}
}

func TestTransactionsMemoArray(t *testing.T) {
	transactions := getTransactions(t, `{
    "transactions": {
        "booked": [
            {
                "RemittanceInformationUnstructured": "foo",
				"transactionAmount": {
                    "amount": "10"
                },
                "bookingDate": "2006-01-02"

            },
            {
                "RemittanceInformationUnstructuredArray": ["foo", "bar"],
				"transactionAmount": {
                    "amount": "10"
                },
                "bookingDate": "2006-01-02"
            }
        ]
    }
}
`)

	want := "foo"
	got := transactions[0].Memo
	if got != want {
		t.Fatalf("failed to parse memo: %s != %s", got, want)
	}

	want = "foo bar"
	got = transactions[1].Memo
	if got != want {
		t.Fatalf("failed to parse memo: %s != %s", got, want)
	}
}
