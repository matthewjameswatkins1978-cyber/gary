# GARY Result Pusher — GitHub App Setup

The GARY control plane uses `actions/create-github-app-token@v2` to obtain a
short-lived installation token. No custom JWT code is needed.

## Browser Steps

### 1. Create the GitHub App
Go to: https://github.com/settings/apps/new

- **GitHub App name:** `GARY Result Pusher`
- **Homepage URL:** `https://github.com/matthewjameswatkins1978-cyber/gary`
- **Webhook URL:** (leave blank)

### 2. Set Permissions
Under "Permissions → Repository permissions":

- Contents: **Read and write**
- Metadata: **Read-only**

Click **Create GitHub App**

### 3. Copy the App ID
Copy the **App ID** (numeric).

Store as repository **variable** (not secret): `GARY_GITHUB_APP_ID`

### 4. Generate Private Key
On the App settings page:
- Scroll to "Private keys"
- Click **Generate a private key**
- Save the downloaded `.pem` file

### 5. Store the Private Key
Store the PEM content as repository **secret**: `GARY_GITHUB_APP_PRIVATE_KEY`

### 6. Install the App
- Go to: https://github.com/settings/apps
- Click **Install** next to "GARY Result Pusher"
- Select repositories:
  - `matthewjameswatkins1978-cyber/gary`
  - `matthewjameswatkins1978-cyber/judgement-call`
  - `matthewjameswatkins1978-cyber/book-burner`
- Click **Install**

## Required Repository Configuration

| Name | Type | Value |
|------|------|-------|
| `GARY_GITHUB_APP_ID` | Variable | App ID (numeric) |
| `GARY_GITHUB_APP_PRIVATE_KEY` | Secret | PEM content (full) |

## How It Works

The `gary-control.yml` workflow uses `actions/create-github-app-token@v2`
with the App ID and private key to generate a short-lived installation token.
This token is passed as `GARY_GITHUB_TOKEN` to the GARY binary for push
operations (submit, recover, status snapshot publishing).

Read-only commands (status, doctor, list, inspect, logs) do not require the
App token. The current master workflow also restricts its short-lived token to
the repositories listed above; add a target repository to that workflow and
install the App on it before dispatching private-repository work.
