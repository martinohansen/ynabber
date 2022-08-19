# Ynabber

Ynabber reads transactions from one or more places and push them into
[YNAB](https://www.youneedabudget.com/).

## Installation

Install [Go](https://go.dev/) and run `go install` to install:

```bash
go install github.com/martinohansen/ynabber/cmd/ynabber@latest
```

## Usage

Ynabber is configured with environment variables, for example reading from
[Nordigen](https://nordigen.com/en/) requires these:

```bash
cat <<EOT >> ynabber.env
# YNAB
YNAB_BUDGETID=<budget_id>
YNAB_TOKEN=<account token>

# Nordigen
NORDIGEN_ACCOUNTMAP='{"<nordigen account id>": "<ynab account id>"}'
NORDIGEN_BANKID=<nordigen bankd id>
NORDIGEN_SECRET_ID=<nordigen secret id>
NORDIGEN_SECRET_KEY=<nordigen secret key>
EOT
```

Run local:

```bash
# Read environment variables from file and run ynabber
export $(grep -v '^#' ynabber.env | xargs) && ynabber
```

Or with Docker:

```bash
docker run --env-file=ynabber.env ghcr.io/martinohansen/ynabber:latest
```

Or deploying to Kubernetes with kubectl:

```bash
kubectl create configmap ynabber-env --from-env-file=ynabber.env
kubectl apply -f kubernetes.yaml
```

## Contributing

Pull requests are welcome.
