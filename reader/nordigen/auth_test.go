package nordigen

import (
	"testing"
)

func TestStore(t *testing.T) {
	auth := Authorization{File: "./ynabber.json"}
	want := "ynabber.json"
	got := auth.Store()
	if want != got {
		t.Fatalf("default: %s != %s", want, got)
	}
}
