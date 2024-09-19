package ynabber

import (
	"testing"
)

func TestMilliunitsFromAmount(t *testing.T) {
	want := Milliunits(123930)
	got := MilliunitsFromAmount(123.93)
	if want != got {
		t.Fatalf("period separated: %s != %s", want, got)
	}

	want = Milliunits(4924340)
	got = MilliunitsFromAmount(4924.34)
	if want != got {
		t.Fatalf("comma separated: %s != %s", want, got)
	}

	want = Milliunits(-2990)
	got = MilliunitsFromAmount(-2.99)
	if want != got {
		t.Fatalf("negative amount: %s != %s", want, got)
	}

	want = Milliunits(-3899000)
	got = MilliunitsFromAmount(-3899)
	if want != got {
		t.Fatalf("amount with no separator: %s != %s", want, got)
	}

	want = Milliunits(328180)
	got = MilliunitsFromAmount(328.18)
	if want != got {
		t.Fatalf("amount with no separator: %s != %s", want, got)
	}

	want = Milliunits(32818000)
	got = MilliunitsFromAmount(32818)
	if want != got {
		t.Fatalf("amount with no separator: %s != %s", want, got)
	}
}
