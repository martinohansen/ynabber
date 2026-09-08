# Enable Banking setup

Create an application in the [Enable Banking dashboard](https://enablebanking.com/)
and download its PEM private key. Register the redirect URL that you will use
with Ynabber. See [Configuration](../../CONFIGURATION.md#enablebanking) for
application, bank, and key settings.

## First authorization

1. Run Ynabber in an interactive terminal.
2. Open the authorization URL it prints and complete the bank login.
3. Paste the full redirect URL at the prompt.

Ynabber saves the session for later runs. The session remains usable until
its expiry or the bank revokes access.

For Docker, enable interactive input and persist the data directory:

```sh
docker run -it --rm \
  --env-file ./ynabber.env \
  -e YNABBER_DATADIR=/data \
  -v "${PWD}/data:/data" \
  ghcr.io/martinohansen/ynabber:latest
```

To authorize an existing container, use `docker attach <container>`. After
pasting the redirect URL, press **Ctrl+P, Ctrl+Q** to detach without stopping it.
