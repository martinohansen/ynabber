package prefix

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/martinohansen/ynabber"
)

func TestProcess(t *testing.T) {
	for _, prefix := range []string{"[test] ", ""} {
		t.Run(prefix, func(t *testing.T) {
			input := []ynabber.Transaction{{Account: ynabber.Account{ID: "account", Name: "Checking", IBAN: "test"}, ID: "transaction", Date: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Payee: "Søby Cafe", Memo: "Lunch", Amount: -12500}, {ID: "empty-payee"}}
			before := slices.Clone(input)
			p := Processor{Config: Config{Prefix: prefix}}
			got, err := p.Process(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			want := slices.Clone(before)
			for i := range want {
				want[i].Payee = prefix + want[i].Payee
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %#v, want %#v", got, want)
			}
			if !reflect.DeepEqual(input, before) {
				t.Fatal("input was changed")
			}
			got[0].Memo = "changed"
			if !reflect.DeepEqual(input, before) {
				t.Fatal("result aliases input")
			}
		})
	}
}

func TestProcessEmptyAndCanceled(t *testing.T) {
	p := Processor{Config: Config{Prefix: "test"}}
	for _, batch := range [][]ynabber.Transaction{nil, {}, {{Payee: "Shop"}}} {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := p.Process(ctx, batch); !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v", err)
		}
		if len(batch) == 0 {
			got, err := p.Process(context.Background(), batch)
			if err != nil || !reflect.DeepEqual(got, batch) {
				t.Fatalf("empty batch: got %#v, error %v", got, err)
			}
		}
	}
}
