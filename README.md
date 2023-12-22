# Ynabber

Ynabber reads and writes transactions from one or more places. For a list of
supported see [readers](#readers) and [writers](#writers).

## Installation

Install [Go](https://go.dev/) and run `go install` to install binary

```bash
go install github.com/martinohansen/ynabber/cmd/ynabber@latest
```

## Usage

Ynabber is configured with environment variables, for example, reading from
[Nordigen](https://nordigen.com/en/) requires these. All valid config options
can be found in the [config.go](config.go) file.

```bash
cat <<EOT >> ynabber.env
# YNAB
YNAB_BUDGETID=<budget_id>
YNAB_TOKEN=<account token>
YNAB_ACCOUNTMAP={"<IBAN>": "<YNAB account ID>"}

# Nordigen
NORDIGEN_BANKID=<nordigen bank ID>
NORDIGEN_SECRET_ID=<nordigen secret ID>
NORDIGEN_SECRET_KEY=<nordigen secret key>
EOT
```

To read the environment variables from a file and run the binary one can use the
[declare](https://www.gnu.org/software/bash/manual/bash.html#index-declare)
command

```bash
# Read environment variables from file and run ynabber
declare $(cat ynabber.env); ynabber
```

Or run the container and parse in the variables with
[Docker](https://docs.docker.com/engine/reference/run/)

```bash
docker run --env-file=ynabber.env ghcr.io/martinohansen/ynabber:latest

# To keep data persistent
docker run \
    --volume ${PWD}:/data \
    --env 'YNABBER_DATADIR=/data' \
    --env-file=ynabber.env \
    ghcr.io/martinohansen/ynabber:latest
```

## Readers

Currently tested readers and verified banks. Any bank supported by
[Nordigen](https://nordigen.com/) (now known as GoCardless) should work.

| Reader   | Bank            |   |
|----------|-----------------|---|
| Nordigen | ALANDSBANKEN_AABAFI22 | ✅
| | NORDEA_NDEADKKK | ✅[^1]
| | NORDEA_NDEAFIHH | ✅
| | NORWEGIAN_FI_NORWNOK1 | ✅
| | S_PANKKI_SBANFIHH | ✅

Please open an [issue](https://github.com/martinohansen/ynabber/issues/new) if
you have problems with a specific bank.

[^1]: Requires setting NORDIGEN_TRANSACTION_ID to "InternalTransactionId"

## Writers

The default writer is YNAB (that's really what this tool is set out to handle)
but we also have a JSON writer that can be used for testing purposes.

| Writer  | Description   |
|---------|---------------|
| YNAB    | Pushes transactions to YNAB |
| JSON    | Writes transactions to stdout in JSON format |

## Contributing

Pull requests are welcome.
