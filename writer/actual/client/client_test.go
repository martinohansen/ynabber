package client

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
		Body:       io.NopCloser(strings.NewReader(`{"data":{"added":["id-1"],"updated":[]}}`)),
		Header:     make(http.Header),
	}, nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestImportTransactions(t *testing.T) {
	transport := &capturingTransport{}
	c := NewClient("https://actual.example.com", "key", "pass", &http.Client{Transport: transport}, nil)

	cleared := true
	tx := []Transaction{{
		Account:    "account-1",
		Date:       "2024-05-10",
		Amount:     1234,
		PayeeName:  "Payee",
		ImportedID: "id-1",
		Cleared:    &cleared,
	}}

	result, err := c.ImportTransactions(context.Background(), "budget-1", "account-1", tx, ImportTransactionsOptions{DefaultCleared: false, ReimportDeleted: true})
	if err != nil {
		t.Fatalf("ImportTransactions() error = %v", err)
	}
	if result.Added != 1 || result.Updated != 0 {
		t.Fatalf("unexpected result %+v", result)
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

	var body importTransactionsRequest
	if err := json.Unmarshal(transport.bodies[0], &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if len(body.Transactions) != 1 {
		t.Fatalf("unexpected transaction count %d", len(body.Transactions))
	}
	if body.DefaultCleared {
		t.Fatalf("expected defaultCleared false")
	}
	if !body.ReimportDeleted {
		t.Fatalf("expected reimportDeleted true")
	}
}

func TestImportTransactionsDryRun(t *testing.T) {
	transport := &capturingTransport{}
	c := NewClient("https://actual.example.com", "", "", &http.Client{Transport: transport}, nil)

	_, err := c.ImportTransactions(context.Background(), "budget-1", "account-1", []Transaction{}, ImportTransactionsOptions{DryRun: true})
	if err != nil {
		t.Fatalf("ImportTransactions() error = %v", err)
	}

	var body importTransactionsRequest
	if err := json.Unmarshal(transport.bodies[0], &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if !body.DryRun {
		t.Fatalf("expected dryRun true")
	}
}

func TestImportTransactionsReturnsImportErrors(t *testing.T) {
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"data":{"added":[],"updated":[],"errors":[{"message":"bad import"}]}}`)),
			Header:     make(http.Header),
		}, nil
	})
	c := NewClient("https://actual.example.com", "key", "pass", &http.Client{Transport: transport}, nil)

	_, err := c.ImportTransactions(context.Background(), "budget-1", "account-1", []Transaction{{
		Account: "account-1",
		Date:    "2024-05-10",
		Amount:  1234,
	}}, ImportTransactionsOptions{})
	if err == nil {
		t.Fatalf("expected import error")
	}
	if !strings.Contains(err.Error(), "bad import") {
		t.Fatalf("expected Actual import error, got %v", err)
	}
}

func TestImportTransactionsReturnsMiddlewareError(t *testing.T) {
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader(`{"error":"Account not found"}`)),
			Header:     make(http.Header),
		}, nil
	})
	c := NewClient("https://actual.example.com", "key", "pass", &http.Client{Transport: transport}, nil)

	_, err := c.ImportTransactions(context.Background(), "budget-1", "account-1", []Transaction{{
		Account: "account-1",
		Date:    "2024-05-10",
		Amount:  1234,
	}}, ImportTransactionsOptions{})
	if err == nil {
		t.Fatalf("expected middleware error")
	}
	if !strings.Contains(err.Error(), "Account not found") {
		t.Fatalf("expected middleware error message, got %v", err)
	}
}

func TestImportTransactionsEscapesPathComponents(t *testing.T) {
	transport := &capturingTransport{}
	c := NewClient("https://actual.example.com", "key", "", &http.Client{Transport: transport}, nil)

	_, err := c.ImportTransactions(context.Background(), "budget/1", "account?2", []Transaction{}, ImportTransactionsOptions{})
	if err != nil {
		t.Fatalf("ImportTransactions() error = %v", err)
	}

	if len(transport.requests) != 1 {
		t.Fatalf("expected one request, got %d", len(transport.requests))
	}
	got := transport.requests[0].URL.EscapedPath()
	want := "/v1/budgets/budget%2F1/accounts/account%3F2/transactions/import"
	if got != want {
		t.Fatalf("expected URL path %q, got %q", want, got)
	}
}
