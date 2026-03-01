# EnableBanking

EnableBanking reads bank transactions through the EnableBanking Open Banking
API. It connects to various European banks using PSD2 open banking standards to
retrieve account information and transaction data.

## Setup

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

## Authentication

On first run, Ynabber will print an authorization URL, wait for you to log in to
your bank, and then ask you to paste the full redirect URL back. Once done, the
session is saved to disk and all future runs are fully non-interactive.

The basic flow:

1. Run Ynabber with an interactive terminal (any method — see below).
2. Visit the printed authorization URL in your browser and log in to your bank.
3. Copy the full redirect URL from your browser's address bar and paste it at the prompt.
4. Ynabber saves the session and starts fetching transactions.

**Note:** Sessions remain valid until the bank revokes them or the `valid_until` timestamp returned by the API (if any) passes.

### Running in Docker

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
