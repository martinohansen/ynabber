package nordigen

import (
	"testing"
)

func TestStore(t *testing.T) {
	r := Reader{
		Config: Config{
			BankID: "foo",
		},
		DataDir: ".",
	}
	want := "foo.json"
	got := r.requisitionStore()
	if want != got {
		t.Fatalf("default: %s != %s", want, got)
	}
}
