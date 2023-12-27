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

All valid config options can be found in the [config.go](config.go) file.

To read the environment variables from a file and run the binary one can use the
[declare](https://www.gnu.org/software/bash/manual/bash.html#index-declare)
command:

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


### Requisition URL Hooks

In order to allow bank account data to flow to YNAB, this application requires an authentication with Nordigen. That URL is called "requistion URL" and is available in the docker logs. For some banks, this access is only valid for 90 days. This application requires a relogin after. In order to make that process easier (i.e. by sending the requistion URL to the phone) ynabber supports hooks when creating a requisition URL. In order to set it up, one first creates a shell-script, for example named `hook.sh`:

```bash
#! /bin/sh

echo "Hi from hook 👋
status: $1
link: $2
at: $(date)"
fi
```

And then configures a hook in the configuration file:
```bash
NORDIGEN_REQUISITION_HOOK=/data/hook.sh
```

When using ynabber throuch docker, keep in mind that the docker container does not support a vast array of command line tools (i.e. no bash, wget instead of cURL).

## Readers

Currently tested readers and verified banks, but any bank supported by Nordigen
should work.

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
