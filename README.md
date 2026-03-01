# Ynabber

Ynabber reads bank transactions from one or more sources (readers) and writes
them to one or more destinations (writers). Use it to pull in your bank
transactions automatically and push them to personal finance apps like YNAB.

See [readers](#readers) and [writers](#writers) below for supported banks and
services.

## Installation

Install with [Go](https://go.dev/),
[Docker](https://www.docker.com/get-started/), or [download the
binary](https://github.com/martinohansen/ynabber/releases). Choose whatever fits
your setup.

```sh
# Go
go install github.com/martinohansen/ynabber/cmd/ynabber@latest

# Docker
docker pull ghcr.io/martinohansen/ynabber:latest
```

## Usage

Ynabber is configured with environment variables. Quickstart with these
examples:

```sh
# YNAB
cat <<EOT > ynabber.env
YNAB_BUDGETID=<budget_id>
YNAB_TOKEN=<account_token>
EOT

# Nordigen/GoCardless
cat <<EOT >> ynabber.env
YNAB_ACCOUNTMAP={"<IBAN>": "<YNAB_account_ID>"}
NORDIGEN_BANKID=<nordigen_bank_ID>
NORDIGEN_SECRET_ID=<nordigen_secret_ID>
NORDIGEN_SECRET_KEY=<nordigen_secret_key>
EOT

# Or EnableBanking
cat <<EOT >> ynabber.env
YNAB_ACCOUNTMAP={"<account id>": "<YNAB_account_ID>"}
ENABLEBANKING_APP_ID=<your_app_id_here>
ENABLEBANKING_COUNTRY=<country code>
ENABLEBANKING_ASPSP=<bank identifier>
ENABLEBANKING_PEM_FILE=<private key pem file>
EOT
```

Run Ynabber locally:

```sh
env $(ynabber.env | xargs) ynabber
```

Or using Docker:

```sh
docker run \
    --volume "${PWD}:/data" \
    --env ‘YNABBER_DATADIR=/data’ \
    --env-file=ynabber.env \
    ghcr.io/martinohansen/ynabber:latest
```

Or as a systemd service:

```sh
sudo cp ynabber.env /etc/ynabber/ynabber.env
sudo cat <<EOT > /etc/systemd/system/ynabber.service
[Unit]
Description=Ynabber
After=network-online.target
Wants=network-online.target

[Service]
EnvironmentFile=/etc/ynabber/ynabber.env
ExecStart=$(which ynabber)
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOT

sudo systemctl enable --now ynabber
```

See [CONFIGURATION.md](./CONFIGURATION.md) for all available settings.

## Readers

Readers fetches transactions from your bank(s) and pushes them to writers.

| Reader | Description |
|:-------|:------------|
| [Nordigen](/reader/nordigen/) | Now known as [GoCardless](https://developer.gocardless.com/bank-account-data/overview/), this is for their "Bank Account Data" product |
| [EnableBanking](/reader/enablebanking/)| Supports lots of financial institutions [across Europe](https://enablebanking.com/docs/markets/) |

## Writers

Writers are destinations for fetched transactions.

| Writer  | Description |
|:--------|:------------|
| [YNAB](/writer/ynab/) | Pushes transactions to a YNAB budget |
| [JSON](/writer/json/) | Writes transactions as JSON to stdout (useful for testing) |

## Contributing

Pull requests welcome. Found a bug or have ideas? [Open an
issue]((https://github.com/martinohansen/ynabber/issues/new)). Help make Ynabber
better for everyone.

_[bitcoin:bc1qct2au09va7rk5psevmesalkaxtjmdjun9x2r3a](bitcoin:bc1qct2au09va7rk5psevmesalkaxtjmdjun9x2r3a)_
