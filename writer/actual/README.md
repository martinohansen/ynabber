# Actual Budget

This writer sends transactions to an [Actual Budget](https://actualbudget.org)
instance via [actual-http-api](https://github.com/jhonderson/actual-http-api),
a third-party REST wrapper around the official Actual JS SDK.

## Configuration

See [Configuration](../../CONFIGURATION.md#actual) for the available Actual
writer settings.

## Notes

- Requires a running [actual-http-api](https://github.com/jhonderson/actual-http-api)
  service. Set `ACTUAL_BASE_URL` to its URL.
- `ACTUAL_ACCOUNTMAP` maps reader account identifiers (IBAN or Account ID) to
  Actual account IDs.
- `ACTUAL_DELAY` can help avoid duplicates if your bank mutates transaction
  data after booking.
- Duplicates are reconciled by Actual using `imported_id`. Repeated IDs in one
  input are placed in separate requests so reconciliation is independent of the
  configured batch limits.
- `ACTUAL_CLEARED` defaults to `false` (transactions are uncleared unless
  configured otherwise).
- `ACTUAL_REIMPORT_DELETED` defaults to `false` so transactions deleted in
  Actual are not imported again unless explicitly configured.
- `ACTUAL_DRY_RUN` simulates the import without persisting any data. Each batch
  is evaluated independently against the unchanged budget, so aggregate counts
  from a multi-batch dry run can differ from a real sequential import.
- Imports are split sequentially per account. `ACTUAL_MAX_REQUEST_BYTES`
  defaults to 80 KiB and limits the exact encoded request body;
  `ACTUAL_BATCH_SIZE` defaults to 100 transactions. Both limits are enforced.
  All batches for an account are planned before its first request, so an
  oversized transaction prevents that account from being partially imported.
- Requests are not atomic across batches. If a request fails, earlier batches
  for the account may already be committed; later batches for that account are
  not attempted, while other accounts continue. A retry is normally safe
  because Actual reconciles transactions by `imported_id`.
- `imported_payee` is sourced from the transaction memo (which contains the raw
  remittance information from the bank) so that Actual's payee-renaming rules
  can match against the full bank text rather than the already-stripped payee
  field. When the memo is empty, the payee is used as a fallback.
- `imported_id` is generated from account, source transaction ID, date, and
  amount when a source ID exists. Transactions without source IDs also include
  payee and memo in the generated ID. For ID generation, IBAN is preferred over
  Account ID for parity with the YNAB writer's hash order, so users running
  both writers see consistent identifiers across budgets. This differs from
  account matching in `ACTUAL_ACCOUNTMAP`, which prefers Account ID over IBAN.

See [actual.go](./actual.go) for implementation details.
