package actual

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type capturingTransport struct {
	requests []*http.Request
	bodies   [][]byte
}

func (c *capturingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	req.Body.Close()
	req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	c.requests = append(c.requests, req)
	c.bodies = append(c.bodies, bodyBytes)
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("{}")),
		Header:     make(http.Header),
	}, nil
}

func TestBatchTransactions(t *testing.T) {
	transport := &capturingTransport{}
	client := NewClient("https://actual.example.com", "key", "pass", &http.Client{Transport: transport}, nil)

	cleared := true
	tx := []Transaction{{
		Account:    "account-1",
		Date:       "2024-05-10",
		Amount:     1234,
		PayeeName:  "Payee",
		ImportedID: "id-1",
		Cleared:    &cleared,
	}}

	if err := client.BatchTransactions(context.Background(), "budget-1", "account-1", tx, Options{RunTransfers: true, LearnCategories: true}); err != nil {
		t.Fatalf("BatchTransactions() error = %v", err)
	}

	if len(transport.requests) != 1 {
		t.Fatalf("expected one request, got %d", len(transport.requests))
	}

	req := transport.requests[0]
	if req.Method != http.MethodPost {
		t.Fatalf("expected POST got %s", req.Method)
	}
	if req.Header.Get("x-api-key") != "key" {
		t.Fatalf("expected api key header")
	}
	if req.Header.Get("budget-encryption-password") != "pass" {
		t.Fatalf("expected encryption header")
	}

	var body addTransactionsRequest
	if err := json.Unmarshal(transport.bodies[0], &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if !body.RunTransfers || !body.LearnCategories {
		t.Fatalf("expected options to be true")
	}
	if len(body.Transactions) != 1 {
		t.Fatalf("unexpected transaction count %d", len(body.Transactions))
	}
}
