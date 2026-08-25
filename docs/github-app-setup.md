# GARY Result Pusher — GitHub App Setup

## Browser Steps (6 steps)

### 1. Create the GitHub App
Go to: https://github.com/settings/apps/new

- **GitHub App name:** `GARY Result Pusher`
- **Homepage URL:** `https://github.com/matthewjameswatkins1978-cyber/gary`
- **Webhook URL:** (leave blank, or use `https://example.com/hook`)
- **Webhook secret:** (leave blank)

### 2. Set Permissions
Under "Permissions":

**Repository permissions:**
- Contents: **Read and write** (needed to push result branches)
- Metadata: **Read-only** (always required)

**Organization permissions:** (none needed)

Click **Create GitHub App**

### 3. Copy the App ID
After creation, copy the **App ID** (numeric, e.g., `12345678`)

Store as repository secret: `GARY_GITHUB_APP_ID`

### 4. Generate Private Key
On the App settings page:
- Scroll to "Private keys"
- Click **Generate a private key**
- Save the downloaded `.pem` file

### 5. Store the Private Key
Store the PEM content as repository secret: `GARY_GITHUB_APP_PRIVATE_KEY`

**Important:** Copy the entire PEM content (including `-----BEGIN RSA PRIVATE KEY-----` and `-----END RSA PRIVATE KEY-----`)

### 6. Install the App
- Go to: https://github.com/settings/apps
- Click **Install** next to "GARY Result Pusher"
- Select repositories:
  - `matthewjameswatkins1978-cyber/gary`
  - `matthewjameswatkins1978-cyber/judgement-call`
- Click **Install**

After installation, copy the **Installation ID** from the URL (the number after `/installations/`)

Store as repository secret: `GARY_GITHUB_APP_INSTALLATION_ID`

---

## Required Repository Secrets

| Secret Name | Value |
|-------------|-------|
| `GARY_GITHUB_APP_ID` | App ID (numeric) |
| `GARY_GITHUB_APP_PRIVATE_KEY` | PEM content (full) |
| `GARY_GITHUB_APP_INSTALLATION_ID` | Installation ID (numeric) |

## Security Notes

- The private key is NEVER logged or committed to Git
- Installation tokens are short-lived (1 hour)
- The App only has access to explicitly installed repositories
- The App only needs Contents: Read/Write permission
