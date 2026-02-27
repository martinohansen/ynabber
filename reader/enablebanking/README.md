# EnableBanking Reader

EnableBanking reads bank transactions through the EnableBanking Open Banking API. It connects to various European banks using PSD2 open banking standards to retrieve account information and transaction data.

## Why EnableBanking

Nordigen was acquired by GoCardless in 2022, and its Open Banking API is now part of GoCardless' bank account data offering. If Nordigen access is unavailable for new sign-ups in your region, EnableBanking is a practical alternative. Source: https://www.openbankingexpo.com/news/gocardless-to-buy-latvian-open-banking-provider-nordigen/

## Setup Guide

### 1. Register for EnableBanking

1. Visit the [EnableBanking website](https://enablebanking.com/)
2. Click **"Sign Up"** or **"Go to Dashboard"**
3. Create an account with your email
4. Verify your email address
5. Log in to the EnableBanking developer dashboard

### 2. Create an Application

1. In the dashboard, navigate to **"Applications"** or **"My Apps"**
2. Click **"Create New Application"**
3. Fill in the application details:
   - **Name**: Choose a descriptive name (e.g., "Ynabber")
   - **Redirect URL**: This is where users will be redirected after authorizing with their bank. You must use this exact URL in your `ENABLEBANKING_REDIRECT_URL` environment variable
   - Example:
     ```
     https://raw.githubusercontent.com/linkmic/ynabber/refs/heads/main/ok.html
     ```
     Or for local testing:
     ```
     https://localhost:8080/callback
     ```
4. Accept the terms and create the application
5. (Recommended) In the app settings, use the built-in key generation to create and download a PEM private key. Save it for later use `ENABLEBANKING_PEM_FILE`.
6. You will receive:
   - **APP_ID**: Save this for `ENABLEBANKING_APP_ID`
   - **API Key** (if applicable)


### 3. Link Bank Accounts

1. In the EnableBanking dashboard, find the **"Link Accounts"** or **"Connect Bank"** section
2. Select your country (e.g., NO for Norway, SE for Sweden)
3. Choose your bank (ASPSP):
   - Tested with **DNB** (Danske Bank Group)
   - Other banks available for your region
4. Follow the bank's authentication flow (online banking login)
5. Grant access to your account(s)
6. The system will confirm the connected accounts

Alternatively, if you prefer to link accounts dynamically during Ynabber setup, the reader will show you a link to authorize during the initial run.

## Configuration

Create an environment file with the following variables:

```bash
cat > enablebanking.env << 'EOF'
# EnableBanking Configuration
ENABLEBANKING_APP_ID=your_app_id_here
ENABLEBANKING_COUNTRY=NO      # Country code: NO, SE, DK, etc.
ENABLEBANKING_ASPSP=DNB        # Bank identifier: DNB, Nordea, SparBank, etc.
ENABLEBANKING_REDIRECT_URL=https://raw.githubusercontent.com/linkmic/ynabber/refs/heads/main/ok.html
ENABLEBANKING_PEM_FILE=./env/private_key.pem  # the file created when setting up the EnableBanking application
# Optional override; otherwise defaults to enablebanking_<aspsp>_<country>_session.json
# ENABLEBANKING_SESSION_FILE=enablebanking_dnb_no_session.json
EOF
```

## Usage

### One-Time Fetch

Run once to fetch transactions from the last configured date:

```bash
# Load environment and run
set -a
. ./enablebanking.env
set +a
ynabber
```

### Continuous Fetching

Set an interval to continuously fetch new transactions:

```bash
# Fetch every 6 hours
ENABLEBANKING_INTERVAL=6h ynabber

# Or set in the env file and run
set -a
. ./enablebanking.env
set +a
ynabber
```

### Initial Authorization

On first run, Ynabber will print an authorization URL, wait for you to log in to your bank, and then ask you to paste the full redirect URL back. Once done, the session is saved to disk and all future runs are fully non-interactive.

The basic flow:

1. Run Ynabber with an interactive terminal (any method — see below).
2. Visit the printed authorization URL in your browser and log in to your bank.
3. Copy the full redirect URL from your browser's address bar and paste it at the prompt.
4. Ynabber saves the session and starts fetching transactions.

**Note:** Sessions remain valid until the bank revokes them or the `valid_until` timestamp returned by the API (if any) passes.

#### Running in Docker

Run the container however you prefer for first-time auth — the simplest approach is to start it interactively:

```sh
docker run -it --rm \
  --env-file ./envs/ynabber_mybank.env \
  -e YNABBER_DATADIR=/data \
  -v ./data/mybank:/data \
  ynabber:latest
```

Paste the redirect URL when prompted. Once `session saved to disk` appears, the session file is in your data volume and you can stop the container.

Alternatively, if the container is already running detached, attach to it:

```sh
docker attach ynabber-mybank
```

Paste the redirect URL, then detach without stopping the container with **Ctrl+P, Ctrl+Q**.

#### Docker Compose

A typical compose service for ongoing (non-interactive) use:

```yaml
services:
  ynabber-mybank:
    image: ynabber:latest
    container_name: ynabber-mybank
    restart: unless-stopped
    stdin_open: true  # required so docker attach can send input if re-auth is ever needed
    tty: true         # required for Ctrl+P Ctrl+Q detach
    user: "1000:1000" # match the owner of the data volume (run `id` to find yours)
    env_file:
      - ./envs/ynabber_mybank.env
    environment:
      - YNABBER_DATADIR=/data
    volumes:
      - ./data/mybank:/data
```

> **User/group ID:** Set `user:` to match whoever owns the data volume on your host so the container can read/write the session file and PEM key. Run `id` to find your UID/GID. If your UID is already 1000 you can omit the `user:` line.

#### Security hardening

The data volume contains sensitive credentials: the session file (live bank API tokens) and your PEM private key. Apply these protections:

```sh
chmod 600 /path/to/private_key.pem   # owner read-only
chmod 700 /path/to/data/mybank        # owner only
```

The container runs as a non-root user (UID 1000) by default. **Never run it as root** (`user: "0:0"` or `user: "root"`).


## Authentication Flow

The reader follows this authentication flow:

1. **Generate JWT Token** - Generates a JWT signed with your private key
2. **Initiate Authorization** - Requests an authorization URL from EnableBanking
3. **User Authorization** - You visit the URL and authorize with your bank
4. **Provide Redirect URL** - You paste the full redirect URL (including `state` parameter) back into the reader
5. **Create Session** - Reader exchanges the code for a persistent session
6. **Fetch Accounts & Transactions** - Uses session to fetch all account and transaction data

Sessions are saved to the path specified in `ENABLEBANKING_SESSION_FILE` (default: `enablebanking_<aspsp>_<country>_session.json` in the current working directory) for subsequent runs.

### Session expiry

The EnableBanking API may return a `valid_until` timestamp in the session response. When present, this is used as the authoritative expiry. When absent, the session is assumed valid until the API rejects it with HTTP 401 — at which point you will see a "session expired" error and must re-authorize.

## Migrating from Nordigen

If you are switching from the Nordigen reader, follow these steps to avoid duplicate transactions in YNAB.

**Before switching:**

1. **Reconcile all accounts in YNAB.** Run the Nordigen reader one final time, approve any pending transactions, and reconcile every account. This gives you a clean baseline.

2. **Note today's date.** This becomes your `ENABLEBANKING_FROM_DATE`. Transactions before this date were already imported by Nordigen; you do not want EnableBanking to re-import them.

3. **Clean up YNAB.** Reject or delete any unapproved/duplicate transactions left over from Nordigen so your accounts are tidy before the cutover.

**Switching over:**

4. Set `ENABLEBANKING_FROM_DATE` to the date from step 2 (format: `YYYY-MM-DD`):
   ```
   ENABLEBANKING_FROM_DATE=2026-02-27
   ```

5. Change `YNABBER_READERS=nordigen` to `YNABBER_READERS=enablebanking` in your env file.

6. Start the EnableBanking reader and complete the authorization flow.

> **Why duplicates happen if you skip this:** Nordigen and EnableBanking assign different internal IDs to the same bank transaction. YNAB's duplicate detection is based on Ynabber's import ID, which includes the transaction ID — so the same transaction imported via both readers looks like two different transactions to YNAB and both will appear.

## Supported Banks

The reader supports any bank implementing PSD2 Open Banking standards through EnableBanking. Currently tested with:

- **DNB** (Norway) - Full support
- **SAS Eurobonus Mastercard** (Norway) - Full support  
- **Sparebanken Øst** (Norway) - Full support

Other banks may be available depending on your region.

## Troubleshooting

### "Session expired"
- The bank rejected the session (HTTP 401) or the `valid_until` timestamp has passed
- Delete the session file and re-run to re-authorize
- Session files are stored at the path specified in `ENABLEBANKING_SESSION_FILE`

### "Bank not found"
- Verify the ASPSP value matches exactly (case-sensitive)
- Check EnableBanking documentation for your region's bank codes

### "No accounts found"
- Ensure the account is linked in your EnableBanking application
- Link additional accounts in the EnableBanking dashboard

## Environment Variables Reference

See [CONFIGURATION.md](/CONFIGURATION.md#enablebanking) for complete details on all configuration options.
