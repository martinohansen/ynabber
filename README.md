# Ynabber

Ynabber reads bank transactions from one or more sources (readers) and writes
them to one or more destinations (writers). Use it to pull in your bank
transactions automatically and push them to personal finance apps like YNAB.

See [readers](#readers) and [writers](#writers) below for supported banks and
services.

## Installation

Install with either [Go](https://go.dev/) or
[Docker](https://www.docker.com/get-started/). Choose what fits your setup.

```sh
# Go
go install github.com/martinohansen/ynabber/cmd/ynabber@latest

# Docker
docker pull ghcr.io/martinohansen/ynabber:latest
```

## Usage

Ynabber is configured via environment variables. Here’s an example setup for
reading transactions from
[GoCardless](https://gocardless.com/bank-account-data/) (formerly Nordigen) and
writing them to YNAB:

```sh
cat <<EOT > ynabber.env
# YNAB
YNAB_BUDGETID=<budget_id>
YNAB_TOKEN=<account_token>
YNAB_ACCOUNTMAP={"<IBAN>": "<YNAB_account_ID>"}

# Nordigen / GoCardless
NORDIGEN_BANKID=<nordigen_bank_ID>
NORDIGEN_SECRET_ID=<nordigen_secret_ID>
NORDIGEN_SECRET_KEY=<nordigen_secret_key>
EOT
```

To run Ynabber with these settings:

```sh
# Load env vars from file and run
set -a
. ./ynabber.env
set +a
ynabber
```

Or using Docker:

```sh
docker run \
    --volume "${PWD}:/data" \
    --env 'YNABBER_DATADIR=/data' \
    --env-file=ynabber.env \
    ghcr.io/martinohansen/ynabber:latest
```

See [CONFIGURATION.md](./CONFIGURATION.md) for all available settings.

## Readers

Readers fetch transactions from your bank. Any bank supported by
[GoCardless](https://gocardless.com/bank-account-data/) should work. Examples
below:

| Reader | Bank | Verified? |
|:-------|:-----|:---------:|
| [Nordigen](/reader/nordigen/)[^1] | ALANDSBANKEN_AABAFI22 | ✅ |
| | NORDEA_NDEADKKK | ✅ |
| | NORDEA_NDEAFIHH | ✅ |
| | NORWEGIAN_FI_NORWNOK1 | ✅ |
| | S_PANKKI_SBANFIHH | ✅ |
| | SPAREBANK_SR_BANK_SPRONO22 | ✅ |

[^1]: Please open an [issue](https://github.com/martinohansen/ynabber/issues/new) if you have problems with a specific bank.

## Writers

Writers are destinations for fetched transactions.

| Writer  | Description   |
|:--------|:--------------|
| [YNAB](/writer/ynab/)    | Pushes transactions to a YNAB budget |
| [JSON](/writer/json/)    | Writes transactions as JSON to stdout (useful for testing) |

## Contributing

Pull requests welcome. Found a bug or have ideas? [Open an
issue]((https://github.com/martinohansen/ynabber/issues/new)). Help make Ynabber
better for everyone.

_[bitcoin:bc1qct2au09va7rk5psevmesalkaxtjmdjun9x2r3a](bitcoin:bc1qct2au09va7rk5psevmesalkaxtjmdjun9x2r3a)_
