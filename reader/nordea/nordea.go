package nordea

import (
	"encoding/csv"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/martinohansen/ynabber"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

func CSVBulkReader() (t []ynabber.Transaction, err error) {
	file, err := os.Open("../input.csv")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	encoding := charmap.Windows1252
	decodedFile := transform.NewReader(file, encoding.NewDecoder())

	reader := csv.NewReader(decodedFile)
	reader.Comma = ';'

	lines, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	timeLayout := "02-01-2006"
	for _, line := range lines[1:] {
		date, err := time.Parse(timeLayout, line[0])
		if err != nil {
			return nil, err
		}

		amount, err := ynabber.AmountParser(line[3], ",")
		if err != nil {
			return nil, err
		}

		payee, err := ynabber.PayeeParser(line[1])
		if err != nil {
			return nil, err
		}

		x := ynabber.Transaction{
			ID:     uuid.New(),
			Date:   date,
			Payee:  payee,
			Memo:   line[1],
			Amount: amount,
		}
		t = append(t, x)
	}
	return t, nil
}
