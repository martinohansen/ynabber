# Ynabber

Ynabber reads transactions from one or more places, called [readers](#readers),
and pushes them into [YNAB](https://www.youneedabudget.com/).

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
YNAB_IMPORT_ID_V2=2023-01-01    # Not required but will give better results

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

Or run the container and parse in the variables with [Docker](https://docs.docker.com/engine/reference/run/)

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

Currently supported readers and verified banks

| Reader   | Bank            |   |
|----------|-----------------|---|
| Nordigen | ALANDSBANKEN_AABAFI22 | ✅
| | NORDEA_NDEADKKK | ✅
| | NORDEA_NDEAFIHH | ✅
| | NORWEGIAN_FI_NORWNOK1 | ✅
| | S_PANKKI_SBANFIHH | ✅

Note that additional banks may also be supported, but they have not been
specifically verified and tested yet. Please open an
[issue](https://github.com/martinohansen/ynabber/issues/new) if you have
problems with a specific bank.

## Contributing

Pull requests are welcome.
