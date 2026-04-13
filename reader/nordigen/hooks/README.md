# Hooks

These shell scripts are example targets for `NORDIGEN_REQUISITION_HOOK`.

Ynabber invokes the hook with `<status> <link>` when a Nordigen requisition
needs user action.

## Examples

- [log-example.sh](./log-example.sh) logs hook calls to `/tmp/nordigen.log`.
- [gmail-example.sh](./gmail-example.sh) emails the link using Gmail SMTP
  credentials from `/data/gmail_config.env`.
- [telegram-example.sh](./telegram-example.sh) sends the link to Telegram.
- [logsnag-example.sh](./logsnag-example.sh) sends the link to Logsnag.
- [fail.sh](./fail.sh) exits non-zero to stop the run in headless workflows.

See [Nordigen](../README.md) and
[Configuration](../../../CONFIGURATION.md#nordigen) for the surrounding setup.
