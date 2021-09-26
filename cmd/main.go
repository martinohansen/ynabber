package main

import (
	"github.com/martinohansen/ynabber/reader/nordigen"
	"github.com/martinohansen/ynabber/writer/ynab"
)

func main() {
	t, err := nordigen.BulkReader()
	if err != nil {
		panic(err)
	}

	err = ynab.BulkWriter(t)
	if err != nil {
		panic(err)
	}
}
