# GARY Result Pusher — GitHub App Setup

GARY uses one trusted GitHub App installation. The control plane exchanges the
App key for a short-lived installation token for the exact repository named by
the accepted project packet. Workers receive only that token; they never
receive the App private key or a reusable PAT.

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

### 6. Install the App on the trusted owner account
- Go to: https://github.com/settings/apps
- Click **Install** next to "GARY Result Pusher"
- Select **All repositories** when the trusted owner account is the intended
  GARY boundary. This lets a newly created private repository work without a
  source-code allowlist edit. If all-repository scope is not acceptable, use
  selected repositories and add each new target in GitHub as a one-time
  installation-scope change.
- Click **Install**
- Record the installation ID from the installation page.

## Required Repository Configuration

| Name | Type | Value |
|------|------|-------|
| `GARY_GITHUB_APP_ID` | Variable | App ID (numeric) |
| `GARY_GITHUB_APP_INSTALLATION_ID` | Variable | Installation ID (numeric) |
| `GARY_GITHUB_APP_PRIVATE_KEY` | Secret | PEM content (full) |

## How It Works

The GARY control plane signs a short-lived App JWT and requests an installation
token with `repositories: [target-repository]`. The token is passed only into
the bounded Batch job as `GARY_GITHUB_TOKEN`. The master workflow uses its
normal ephemeral workflow token only to publish status back to the GARY
control repository.

Read-only commands (status, doctor, list, inspect, logs) do not require the
App token. Before a private target can run, its URL must be an HTTPS GitHub
repository under the approved owner `matthewjameswatkins1978-cyber`. The repo
name is not hardcoded. GARY rejects other owners, malformed URLs, missing App
configuration, and failed token minting before worker submission.

## Adding a new private repository

Create the repository under the approved owner and ensure the existing App
installation covers it. With **All repositories** selected, this is automatic;
with selected-repository scope, add it to the App installation once. Then use
its canonical `.git` URL in the project packet. No GARY workflow or source-code
edit is required.
