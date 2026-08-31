package wealthreader

import "encoding/json"

// Response is the envelope returned by POST /token/ and POST /entities/.
// Same schema as the iframe callback: success + payload + statistics.
// See https://www.wealthreader.com/docs/en/iframe-backend/ and OpenAPI 8.1.7.
type Response struct {
	Success    bool       `json:"success"`
	Payload    Payload    `json:"payload"`
	Statistics Statistics `json:"statistics"`
	Error      *APIError  `json:"error"`
}

// Payload holds the normalised bank data. Keys depend on product_types;
// this reader maps accounts (and optionally other products later).
type Payload struct {
	UserInformation json.RawMessage `json:"user_information"`
	Accounts        []Account       `json:"accounts"`
}

// Account is a Wealth Reader current/savings/… account.
type Account struct {
	UUID         string          `json:"uuid"`
	Subtype      string          `json:"subtype"`
	Code         string          `json:"code"` // IBAN or local account number
	Name         string          `json:"name"`
	Currency     string          `json:"currency"`
	Balances     json.RawMessage `json:"balances"`
	Transactions []Transaction   `json:"transactions"`
}

// Transaction is a Wealth Reader account movement.
// Amount is json.Number so we can feed MilliunitsFromString without float drift.
type Transaction struct {
	UUID            string          `json:"uuid"`
	ValueDate       string          `json:"value_date"`
	OperationDate   string          `json:"operation_date"`
	Amount          json.Number     `json:"amount"`
	Balance         json.Number     `json:"balance"`
	Description     string          `json:"description"`
	TransferDetails TransferDetails `json:"transfer_details"`
}

// TransferDetails is present on SEPA-like movements.
type TransferDetails struct {
	SenderReceiver string `json:"sender_receiver"`
	AccountNumber  string `json:"account_number"`
	Concept        string `json:"concept"`
}

// Statistics carries the values we persist after a successful read.
type Statistics struct {
	Session     string          `json:"SESSION"`
	OperationID string          `json:"operation_id"`
	Token       string          `json:"token"`
	Code        string          `json:"code"`
	Warnings    json.RawMessage `json:"warnings"`
}

// APIError is the error object Wealth Reader returns with success=false.
// Numeric codes are documented at https://api.wealthreader.com/error-codes/.
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
