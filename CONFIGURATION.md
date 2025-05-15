# Configuration

This document is generated from configuration structs in the source code using `go generate`. **Do not edit manually.**

## Config

| Environment variable | Type | Default | Description |
|:---------------------|:-----|:--------|:------------|
| YNABBER_DATADIR | `string` | `.` | DataDir is the path for storing files |
| YNABBER_DEBUG | `bool` | `false` | Debug prints more log statements |
| YNABBER_INTERVAL | `time.Duration` | `6h` | Interval is how often to execute the read/write loop, 0=run only once |
| YNABBER_READERS | `[]string` | `nordigen` | Readers is a list of sources to read transactions from. Currently only<br>Nordigen is supported. |
| YNABBER_WRITERS | `[]string` | `ynab` | Writers is a list of destinations to write transactions to. |

## Nordigen

| Environment variable | Type | Default | Description |
|:---------------------|:-----|:--------|:------------|
| NORDIGEN_BANKID | `string` | - | BankID is used to create requisition |
| NORDIGEN_SECRET_ID | `string` | - | SecretID is used to create requisition |
| NORDIGEN_SECRET_KEY | `string` | - | SecretKey is used to create requisition |
| NORDIGEN_PAYEE_SOURCE | `PayeeSources` | `unstructured,name,additional` | PayeeSource is a list of sources for Payee candidates, the first method<br>that yields a result will be used. Valid options are: unstructured, name<br>and additional.<br><br>* unstructured: uses the `RemittanceInformationUnstructured` field<br>* name: uses either the either `debtorName` or `creditorName` field<br>* additional: uses the `AdditionalInformation` field<br><br>The sources can be combined with the "+" operator. For example:<br>"name+additional,unstructured" will combine name and additional into a<br>single Payee or use unstructured if both are empty. |
| NORDIGEN_PAYEE_STRIP | `[]string` | - | PayeeStrip is a list of words to remove from Payee. For example:<br>"foo,bar" |
| NORDIGEN_TRANSACTION_ID | `string` | `TransactionId` | TransactionID is the field to use as transaction ID. Not all banks use<br>the same field and some even change the ID over time.<br><br>Valid options are: TransactionId, InternalTransactionId |
| NORDIGEN_REQUISITION_HOOK | `string` | - | RequisitionHook is a exec hook thats executed at various stages of the<br>requisition process. The hook is executed with the following arguments:<br><status> <link> |
| NORDIGEN_REQUISITION_FILE | `string` | - | RequisitionFile overrides the file used to store the requisition. This<br>file is placed inside the YNABBER_DATADIR. |

## YNAB

| Environment variable | Type | Default | Description |
|:---------------------|:-----|:--------|:------------|
| YNAB_BUDGETID | `string` | - | BudgetID for the budget you want to import transactions into. You can<br>find the ID in the URL of YNAB: https://app.youneedabudget.com/<budget_id>/budget |
| YNAB_TOKEN | `string` | - | Token is your personal access token as obtained from the YNAB developer<br>settings section |
| YNAB_ACCOUNTMAP | `AccountMap` | - | AccountMap of IBAN to YNAB account IDs in JSON. For example:<br>'{"<IBAN>": "<YNAB Account ID>"}' |
| YNAB_FROM_DATE | `Date` | - | FromDate only import transactions from this date and onward. For<br>example: 2006-01-02 |
| YNAB_DELAY | `time.Duration` | `0` | Delay sending transaction to YNAB by this duration. This can be necessary<br>if the bank changes transaction IDs after some time. Default is 0 (no<br>delay). |
| YNAB_CLEARED | `TransactionStatus` | `uncleared` | Set cleared status, possible values: cleared, uncleared, reconciled .<br>Default is uncleared for historical reasons but recommend setting this<br>to cleared because ynabber transactions are cleared by bank.<br>They'd still be unapproved until approved in YNAB. |
| YNAB_SWAPFLOW | `[]string` | - | SwapFlow changes inflow to outflow and vice versa for any account with a<br>IBAN number in the list. This maybe be relevant for credit card accounts.<br><br>Example: "DK9520000123456789,NO8330001234567" |

