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
# AccountMap can use IBAN or Account ID (enablebanking account_uid)
YNAB_ACCOUNTMAP={"<IBAN_or_account_id>": "<YNAB_account_ID>"}

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

> **Note for EnableBanking users:** The initial OAuth authorization requires
> interactive terminal input. Run the container with `-it` for the first run
> (see [reader/enablebanking/README.md](./reader/enablebanking/README.md#running-in-docker)).
> Subsequent runs are fully non-interactive.

See [CONFIGURATION.md](./CONFIGURATION.md) for all available settings.

## Readers

Readers fetch transactions from your bank using PSD2 Open Banking standards.

| Reader | Documentation | Verified Banks |
|:-------|:--------------|:---------------:|
| [Nordigen](/reader/nordigen/)[^1] | [Setup Guide](/reader/nordigen/README.md) | ALANDSBANKEN, NORDEA, S_PANKKI, SPAREBANK |
| [EnableBanking](/reader/enablebanking/)[^2] *(Experimental)* | [Setup Guide](/reader/enablebanking/README.md) | DNB, Sbanken, SAS Eurobonus Mastercard, and others |

[^1]: Connected through GoCardless. Please open an [issue](https://github.com/martinohansen/ynabber/issues/new) if you have problems with a specific bank.
[^2]: Connected through EnableBanking Open Banking API. Supports any bank implementing PSD2. See [known limitations](reader/enablebanking/README.md#experimental).

### EnableBanking Setup (Short Version)

1. Register at [EnableBanking](https://enablebanking.com/) and create an application.
2. Use EnableBanking's app setup to generate and download the PEM key (recommended), or generate an RSA PKCS8 private key (PEM) and upload the public key to your app.
3. Link bank accounts in the EnableBanking dashboard.
4. Set `ENABLEBANKING_APP_ID`, `ENABLEBANKING_COUNTRY`, `ENABLEBANKING_ASPSP`, `ENABLEBANKING_REDIRECT_URL`, and `ENABLEBANKING_PEM_FILE`.

See the full guide in [reader/enablebanking/README.md](reader/enablebanking/README.md).

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
