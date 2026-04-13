# YNAB

This writer pushes transactions to a YNAB budget using the YNAB API.

## Configuration

See [Configuration](../../CONFIGURATION.md#ynab) for the available YNAB writer
settings.

## Notes

- `YNAB_ACCOUNTMAP` maps reader account identifiers to YNAB account IDs.
- `YNAB_DELAY` can help avoid duplicates if your bank mutates transaction data
  after booking.

See [ynab.go](./ynab.go) for implementation details.
