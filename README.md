# Ynabber

Ynabber reads transactions from one or more places and push them into [YNAB](https://www.youneedabudget.com/).

## Installation

Install [Go](https://go.dev/) and run `go get` to install:

```bash
go get github.com/martinohansen/ynabber
```

## Usage

Ynabber is configured with enviornment variables, for example reading from [Nordigen](https://nordigen.com/en/) requires this:

```bash
# YNAB
YNAB_BUDGETID: <budget_id>
YNAB_TOKEN: <account token>

# Nordigen
NORDIGEN_ACCOUNTMAP: {"<nordigen account id>": "<ynab account id>"}
NORDIGEN_BANKID: <nordigen bankd id>
NORDIGEN_SECRET_ID: <nordigen secret id>
NORDIGEN_SECRET_KEY: <nordigen secret key>
```

Run local:

```bash
ynabber
```

Or with Docker:

```bash
docker run ghcr.io/martinohansen/ynabber:latest
```

Or deploying to Kubernetes with kubectl:

```bash
kubectl apply -f kubernetes.yaml
```

## Contributing
Pull requests are welcome.
