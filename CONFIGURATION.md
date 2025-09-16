# Configuration

This document is generated from configuration structs in the source code using `go generate`. **Do not edit manually.**

## Ynabber

Ynabber moves transactions from reader to writer in a fan-out fashion. Every writer will receive all transactions from all readers.

| Environment variable | Type | Default | Description |
|:---------------------|:-----|:--------|:------------|
| YNABBER_DATADIR | `string` | `.` | DataDir is the path for storing files |
| YNABBER_LOG_LEVEL | `string` | `info` | LogLevel sets the logging level (error, warn, info, debug, trace) |
| YNABBER_LOG_FORMAT | `string` | `text` | LogFormat sets the logging format (text, json) |
| YNABBER_READERS | `[]string` | `nordigen` | Readers is a list of sources to read transactions from. Currently only<br>Nordigen is supported. |
| YNABBER_WRITERS | `[]string` | `ynab` | Writers is a list of destinations to write transactions to. |

## Nordigen

Nordigen reads bank transactions through the Nordigen/GoCardless API. It connects to various European banks using PSD2 open banking standards to retrieve account information and transaction data.

| Environment variable | Type | Default | Description |
|:---------------------|:-----|:--------|:------------|
| NORDIGEN_BANKID | `string` | - | BankID identifies the bank for creating requisitions |
| NORDIGEN_SECRET_ID | `string` | - | SecretID is the client ID for API authentication |
| NORDIGEN_SECRET_KEY | `string` | - | SecretKey is the client secret for API authentication |
| NORDIGEN_PAYEE_SOURCE | `PayeeGroups` | `remittance,name,additional` | PayeeSource defines the sources and order for extracting payee<br>information. Multiple sources can be combined with "+" to merge their<br>values. Groups are separated by "," and tried in order until a non-empty<br>result is found.<br><br>Available sources:<br>* remittance: uses the remittanceInformation fields<br>* name: uses either the debtorName or creditorName field<br>* additional: uses the additionalInformation field<br><br>Example: "name+additional,remittance" will first try to combine name and<br>additional fields, falling back to remittance if both are empty. |
| NORDIGEN_PAYEE_STRIP | `[]string` | - | PayeeStrip contains words to remove from payee names.<br>Example: "foo,bar" removes "foo" and "bar" from all payee names. |
| NORDIGEN_TRANSACTION_ID | `string` | `TransactionId` | TransactionID specifies which field to use as the unique transaction<br>identifier. Banks may use different fields, and some change the ID format<br>over time.<br><br>Valid options: TransactionId, InternalTransactionId,<br>ProprietaryBankTransactionCode |
| NORDIGEN_REQUISITION_HOOK | `string` | - | RequisitionHook is an executable that runs at various stages of the<br>requisition process. It receives arguments: &lt;status&gt; &lt;link&gt;<br>Non-zero exit codes will stop the process. |
| NORDIGEN_REQUISITION_FILE | `string` | - | RequisitionFile specifies the filename for storing requisition data.<br>The file is stored in the directory defined by YNABBER_DATADIR. |
| NORDIGEN_INTERVAL | `time.Duration` | `6h` | Interval determines how often to fetch new transactions.<br>Set to 0 to run only once instead of continuously. |

## Actual

Package actual provides a writer implementation that sends transactions to an Actual Budget HTTP API instance.

| Environment variable | Type | Default | Description |
|:---------------------|:-----|:--------|:------------|
| ACTUAL_BASE_URL | `string` | - | BaseURL points to the running actual-http-api service, e.g. https://actual.example.com |
| ACTUAL_API_KEY | `string` | - | APIKey is an optional shared secret that will be sent via the x-api-key header. |
| ACTUAL_BUDGET_ID | `string` | - | BudgetID is the Actual Sync ID for the budget to update. |
| ACTUAL_ACCOUNTMAP | `AccountMap` | - | AccountMap maps IBAN numbers to Actual account identifiers. |
| ACTUAL_ENCRYPTION_PASSWORD | `string` | - | EncryptionPassword optionally unlocks end-to-end encrypted budgets. |
| ACTUAL_FROM_DATE | `Date` | - | FromDate skips transactions older than this date when provided. |
| ACTUAL_DELAY | `time.Duration` | `0` | Delay prevents sending very recent transactions that might still change. |
| ACTUAL_RUN_TRANSFERS | `bool` | `false` | RunTransfers instructs Actual to automatically create transfers. |
| ACTUAL_LEARN_CATEGORIES | `bool` | `false` | LearnCategories updates rules based on the incoming category field. |
| ACTUAL_CLEARED | `bool` | `false` | Cleared toggles the cleared flag for newly created transactions. |

## Ynab

YNAB writes transactions You Need a Budget (YNAB) using their API. It handles transaction and account mapping, validation, deduplication, inflow/outflow swapping, and transaction filtering.

| Environment variable | Type | Default | Description |
|:---------------------|:-----|:--------|:------------|
| YNAB_BUDGETID | `string` | - | BudgetID for the budget you want to import transactions into. You can<br>find the ID in the URL of YNAB: https://app.youneedabudget.com/&lt;budget_id&gt;/budget |
| YNAB_TOKEN | `string` | - | Token is your personal access token obtained from the YNAB developer<br>settings section |
| YNAB_ACCOUNTMAP | `AccountMap` | - | AccountMap maps IBANs to YNAB account IDs in JSON format. For example:<br>'{"&lt;IBAN&gt;": "&lt;YNAB Account ID&gt;"}' |
| YNAB_FROM_DATE | `Date` | - | FromDate only imports transactions from this date onward. For<br>example: 2006-01-02 |
| YNAB_DELAY | `time.Duration` | `0` | Delay sending transactions to YNAB by this duration. This can be<br>necessary if the bank changes transaction IDs after some time. Default is<br>0 (no delay). |
| YNAB_CLEARED | `TransactionStatus` | `cleared` | Cleared sets the transaction status. Possible values: cleared, uncleared,<br>reconciled. |
| YNAB_SWAPFLOW | `[]string` | - | SwapFlow reverses inflow to outflow and vice versa for any account with<br>an IBAN number in the list. This may be relevant for credit card<br>accounts.<br><br>Example: "DK9520000123456789,NO8330001234567" |

