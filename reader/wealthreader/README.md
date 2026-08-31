# Wealth Reader

Wealth Reader is a European AIS aggregator (Spain and other EU markets). This
reader talks to the public API documented at
<https://www.wealthreader.com/docs/en/>.

It is **not** a replacement for GoCardless; it is an additional aggregator, the
same way Enable Banking is.

## Setup

### 1. Get an API key

1. Sign up at <https://www.wealthreader.com/> and complete onboarding.
2. Copy the `api_key` from the [client area](https://www.wealthreader.com/clients/).
3. Book the technical onboarding session if you have not already (required by Wealth Reader).

### 2. Register the OAuth redirect URL

The redirect URL must be **exactly** the value of `WEALTHREADER_REDIRECT_URL`.
Register it with `access_type=oauth` and tokenisation on:

```bash
curl --location 'https://api.wealthreader.com/domains/' \
  --header 'Content-Type: application/x-www-form-urlencoded' \
  --data-urlencode 'method=add' \
  --data-urlencode 'api_key=YOUR_API_KEY' \
  --data-urlencode 'domain=https://example.com/oauth/success' \
  --data-urlencode 'url_callback=https://example.com/oauth/success' \
  --data-urlencode 'access_type=oauth' \
  --data-urlencode 'tokenize=1'
```

`tokenize=1` is required: later refreshes use `statistics.token`, not the user's
password.

You can use the same GitHub Pages helper as Enable Banking
(`https://martinohansen.github.io/ynabber/ok.html`) if you register that URL.

### 3. Pick an institution

```bash
curl 'https://api.wealthreader.com/entities/'
```

Set `WEALTHREADER_CODE` to the institution `code` (`bbva`, `caixabank`, …).

### 4. Environment

```sh
YNABBER_READERS=wealthreader
WEALTHREADER_API_KEY=<api_key>
WEALTHREADER_CODE=<institution code>
WEALTHREADER_REDIRECT_URL=https://example.com/oauth/success
WEALTHREADER_FROM_DATE=2024-01-01
```

Account matching in YNAB/Actual uses the Wealth Reader account `uuid` (as
`YNAB_ACCOUNTMAP` key) or the IBAN in `code`.

## Authentication

On first run Ynabber prints the Wealth Reader OAuth URL, waits for you to log
in, and asks you to paste the full redirect URL (`?nonce=…&code=…`). The token
is saved under `YNABBER_DATADIR` and later runs are non-interactive:

```
POST https://api.wealthreader.com/entities/
  api_key + code + token + date_from + product_types
```

Sandbox users from the Wealth Reader docs:

| Username   | Result                          |
|------------|---------------------------------|
| `MOCKDATA` | Successful anonymised read      |
| `MOCKOTP`  | Recreates a 2FA challenge       |
| `MOCKLOGINKO` | Recreates a login error      |

If the token stops working (password change or new 2FA), delete the session
file and run again so the user can re-authenticate.
