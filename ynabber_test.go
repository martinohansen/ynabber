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
}

func TestPayeeParsed(t *testing.T) {
	want := "Im just alphanumeric"
	got, _ := Payee("Im just alphanumeric").Parsed([]string{})
	if want != got {
		t.Fatalf("alphanumeric: %s != %s", want, got)
	}

	want = "你好世界"
	got, _ = Payee("你好世界").Parsed([]string{})
	if want != got {
		t.Fatalf("non-english: %s != %s", want, got)
	}

	want = "Im here"
	got, _ = Payee("Im not here").Parsed([]string{"not "})
	if want != got {
		t.Fatalf("strip words: %s != %s", want, got)
	}

	want = "Im not j ust alphanumeric"
	got, _ = Payee("Im! not j.ust alphanumeric42 69").Parsed([]string{})
	if want != got {
		t.Fatalf("non-alphanumeric: %s != %s", want, got)
	}
}
