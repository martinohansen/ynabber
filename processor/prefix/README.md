# Prefix processor

The `prefix` processor prepends `YNABBER_PREFIX_PAYEE_PREFIX` to every payee.
The default prefix is `[test] `, including the trailing space. An explicitly
empty prefix leaves payees unchanged.

Test it locally without bank credentials:

```sh
YNABBER_READERS=generator \
YNABBER_WRITERS=json \
YNABBER_PROCESSORS=prefix \
YNABBER_PREFIX_PAYEE_PREFIX='[test] ' \
go run ./cmd/ynabber
```
