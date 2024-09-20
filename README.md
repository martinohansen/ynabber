# Ynabber

Ynabber sets out to read and write bank transactions from one or more sources
(known as readers) to one or more destinations (known as writers).

For a list of supported see the [readers](#readers) and [writers](#writers)
subsections.

## Installation

Install [Go](https://go.dev/) and run `go install` to install binary

```bash
go install github.com/martinohansen/ynabber/cmd/ynabber@latest
```

## Usage

Ynabber is configured with environment variables. To read from
[Nordigen](https://nordigen.com/en/) (now known as GoCardless) and write to YNAB
use these values:

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

_All valid config options can be found [here](https://pkg.go.dev/github.com/martinohansen/ynabber#Config)_

Once the environment variables are set, run the binary:

```bash
# Read environment variables from file first
set -a; . ./ynabber.env; set +a; ynabber
ynabber
```

Or run in a container with
[Docker](https://docs.docker.com/engine/reference/run/):


```bash
docker run \
    --volume ${PWD}:/data \
    --env 'YNABBER_DATADIR=/data' \
    --env-file=ynabber.env \
    ghcr.io/martinohansen/ynabber:latest
```

## Readers

Currently tested readers and verified banks, but any bank supported by Nordigen
should work.

| Reader   | Bank            |   |
|----------|-----------------|---|
| [Nordigen](/reader/nordigen/)[^1] | ALANDSBANKEN_AABAFI22 | ✅
| | NORDEA_NDEADKKK | ✅
| | NORDEA_NDEAFIHH | ✅
| | NORWEGIAN_FI_NORWNOK1 | ✅
| | S_PANKKI_SBANFIHH | ✅

[^1]: Please open an [issue](https://github.com/martinohansen/ynabber/issues/new) if
you have problems with a specific bank.

## Writers

The default writer is YNAB (that's really what this tool is set out to handle)
but we also have a JSON writer that can be used for testing purposes.

| Writer  | Description   |
|---------|---------------|
| [YNAB](/writer/ynab/)    | Pushes transactions to YNAB |
| [JSON](/writer/json/)    | Writes transactions to stdout in JSON format |

## Contributing

Pull requests are welcome.
