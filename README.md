# Ynabber

Ynabber is a tool that reads and writes bank transactions from one or more
sources (called readers) to one or more destinations (called writers). This
makes it easier to pull in your bank transactions automatically and push them to
personal finance apps like YNAB.

See subsection [readers](#readers) and [writers](#writers) for a list of
supported banks and services.


## Installation

You can use [Go](https://go.dev/) or
[Docker](https://www.docker.com/get-started/) to install and run Ynabber. Choose
whichever option you find more convenient.

```bash
# Install with Go
go install github.com/martinohansen/ynabber/cmd/ynabber@latest

# Install with Docker
docker pull ghcr.io/martinohansen/ynabber:latest
```

## Usage

Ynabber is configured via environment variables. Below is an example setup for
reading transactions from
[GoCardless](https://gocardless.com/bank-account-data/) (formerly known as
Nordigen) and writing them to YNAB.

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

Then run Ynabber:

```bash
# Load environment variables from the file
set -a; . ./ynabber.env; set +a; ynabber

# Then run
ynabber
```

Or using Docker:

```bash
docker run \
    --volume ${PWD}:/data \
    --env 'YNABBER_DATADIR=/data' \
    --env-file=ynabber.env \
    ghcr.io/martinohansen/ynabber:latest
```

_All valid config variables can be found [here](https://pkg.go.dev/github.com/martinohansen/ynabber#Config)_

## Readers

Readers are how Ynabber fetches your transactions from the bank. Below are some
tested examples. Generally, any bank supported by
[GoCardless](https://gocardless.com/bank-account-data/) (formerly known as
Nordigen) should work:

| Reader | Bank | Verified? |
|:-------|:-----|:---------:|
| [Nordigen](/reader/nordigen/)[^1] | ALANDSBANKEN_AABAFI22 | ✅ |
| | NORDEA_NDEADKKK | ✅ |
| | NORDEA_NDEAFIHH | ✅ |
| | NORWEGIAN_FI_NORWNOK1 | ✅ |
| | S_PANKKI_SBANFIHH | ✅ |

[^1]: Please open an [issue](https://github.com/martinohansen/ynabber/issues/new) if
you have problems with a specific bank.

## Writers

Writers tell Ynabber where to send the fetched transactions.

| Writer  | Description   |
|:--------|:--------------|
| [YNAB](/writer/ynab/)    | Pushes transactions to a YNAB budget |
| [JSON](/writer/json/)    | Writes transactions as JSON to stdout (useful for testing) |

## Contributing

Pull requests are welcome! If you encounter a bug or have an idea for
improvement, feel free to [open an issue](https://github.com/martinohansen/ynabber/issues/new).
We’d love your help in making Ynabber better for everyone!
