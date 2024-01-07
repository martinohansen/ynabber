# Nordigen

This reader reads transactions from [Nordigen](https://nordigen.com/en/), now
acquired by GoCardless.

## Requisition Hook

In order to allow bank account data to flow, you must be authenticated to your
bank. To authenticate a requisition URL is available in the logs. For some banks
this access is only valid for 90 days, whereafter a reauthorization is required.
To ease that process a requisition hook is made available.

See [config.go](../../config.go) for information on how to configure it.

### Examples

A few shell scripts that can be used as targets for the hook are available in
the [hooks](./hooks/) directory.
