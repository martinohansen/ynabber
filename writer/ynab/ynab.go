package ynab

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/google/uuid"
	"github.com/martinohansen/ynabber"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

func BulkWriter(ctx context.Context, t []ynabber.Transaction) error {
	tracer := otel.Tracer("ynabber")
	spanName := "BulkWriter"
	_, span := tracer.Start(ctx, spanName)

	budgetID, found := os.LookupEnv("YNAB_BUDGETID")
	if !found {
		span.End()
		return fmt.Errorf("env variable YNAB_BUDGETID: %w", ynabber.ErrNotFound)
	}
	token, found := os.LookupEnv("YNAB_TOKEN")
	if !found {
		span.End()
		return fmt.Errorf("env variable YNAB_TOKEN: %w", ynabber.ErrNotFound)
	}

	// Read a list of payee strings to strip from env
	var strips []string
	stripConfig := ynabber.ConfigLookup("YNABBER_PAYEE_STRIP", "[]")
	err := json.Unmarshal([]byte(stripConfig), &strips)
	if err != nil {
		span.End()
		return fmt.Errorf("env variable YNABBER_PAYEE_STRIP: %w", err)
	}

	if len(t) == 0 {
		log.Println("No transactions to write")
		span.End()
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
		payee, err := v.Payee.Parsed(strips)
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
	span.SetAttributes(attribute.Int("transactions.count", len(y.Transactions)))

	url := fmt.Sprintf("https://api.youneedabudget.com/v1/budgets/%s/transactions", budgetID)

	payload, err := json.Marshal(y)
	if err != nil {
		span.End()
		return err
	}

	client := &http.Client{}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		span.End()
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	res, err := client.Do(req)
	if err != nil {
		span.End()
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		span.End()
		return fmt.Errorf("failed to send request: %s", res.Status)
	} else {
		log.Printf("Successfully sent %v transaction(s) to YNAB", len(y.Transactions))
	}
	span.End()
	return nil
}
