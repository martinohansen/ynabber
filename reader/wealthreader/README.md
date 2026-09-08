# Wealth Reader setup

Obtain an API key through [Wealth Reader onboarding](https://help.wealthreader.com/).
See [Configuration](../../CONFIGURATION.md#wealthreader) for Ynabber settings.

## Register the OAuth redirect

Register the URL you will use with Ynabber. Enable tokenization so later runs
can use a saved token:

```sh
curl 'https://api.wealthreader.com/domains/' \
  --data-urlencode 'method=add' \
  --data-urlencode 'api_key=YOUR_API_KEY' \
  --data-urlencode 'domain=https://example.com/oauth/success' \
  --data-urlencode 'url_callback=https://example.com/oauth/success' \
  --data-urlencode 'access_type=oauth' \
  --data-urlencode 'tokenize=1'
```

Find institution codes with `curl 'https://api.wealthreader.com/entities/'`.
Use the account UUID or IBAN when mapping accounts to a writer.

## First authorization

Run Ynabber interactively, open the printed OAuth URL, and complete the login.
Paste the full redirect URL, including `nonce` and `code`, at the prompt.
Later runs use the saved token. If a password change or a new authentication
challenge makes the token unusable, delete the session file and authorize again.

Sandbox login names are `MOCKDATA` for a successful read, `MOCKOTP` for an
additional authentication challenge, and `MOCKLOGINKO` for a login error.
