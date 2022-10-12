package ynab

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"os"

	"github.com/google/uuid"
	"github.com/martinohansen/ynabber"
)

func BulkWriter(cfg ynabber.Config, t []ynabber.Transaction) error {
	budgetID, found := os.LookupEnv("YNAB_BUDGETID")
	if !found {
		return fmt.Errorf("env variable YNAB_BUDGETID: %w", ynabber.ErrNotFound)
	}
	token, found := os.LookupEnv("YNAB_TOKEN")
	if !found {
		return fmt.Errorf("env variable YNAB_TOKEN: %w", ynabber.ErrNotFound)
	}

	if len(t) == 0 {
		log.Println("No transactions to write")
		return nil
	}

	type Ytransaction struct {
		AccountID string `json:"account_id"`
		Date      string `json:"date"`
		Amount    string `json:"amount"`
		PayeeName string `json:"payee_name"`
		Memo      string `json:"memo"`
		ImportID  string `json:"import_id"`
	}
	type Ytransactions struct {
		Transactions []Ytransaction `json:"transactions"`
	}

	y := new(Ytransactions)
	for _, v := range t {
		date := v.Date.Format("2006-01-02")
		amount := v.Amount.String()
		payee, err := v.Payee.Parsed(cfg.YNAB.PayeeStrip)
		if err != nil {
			payee = string(v.Payee)
			log.Printf("Failed to parse payee: %s: %s", payee, err)
		}

		// Generating YNAB compliant import ID, output example:
		// YBBR:-741000:2021-02-18:92f2beb1
		hash := sha256.Sum256([]byte(v.Memo))
		id := fmt.Sprintf("YBBR:%s:%s:%x", amount, date, hash[:2])

		x := Ytransaction{
			AccountID: uuid.UUID(v.Account.ID).String(),
			Date:      date,
			Amount:    amount,
			PayeeName: payee,
			Memo:      v.Memo,
			ImportID:  id,
		}

		y.Transactions = append(y.Transactions, x)
	}

	url := fmt.Sprintf("https://api.youneedabudget.com/v1/budgets/%s/transactions", budgetID)

	payload, err := json.Marshal(y)
	if err != nil {
		return err
	}

	client := &http.Client{}

	if cfg.Debug {
		log.Printf("Request: %s\n", payload)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if cfg.Debug {
		b, _ := httputil.DumpResponse(res, true)
		log.Printf("Response: %s\n", b)
	}

	if res.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to send request: %s", res.Status)
	} else {
		log.Printf("Successfully sent %v transaction(s) to YNAB", len(y.Transactions))
	}
	return nil
}
