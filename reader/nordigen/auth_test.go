package nordigen

import (
	"testing"

	"github.com/martinohansen/ynabber"
)

func TestStore(t *testing.T) {
	r := Reader{
		Config: &ynabber.Config{
			Nordigen: ynabber.Nordigen{
				BankID: "foo",
			},
			DataDir: ".",
		},
	}
	want := "foo.json"
	got := r.requisitionStore()
	if want != got {
		t.Fatalf("default: %s != %s", want, got)
	}
}
