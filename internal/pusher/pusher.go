// Package pusher manages GitHub App authentication for pushing result branches.
package pusher

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// AppConfig holds the GitHub App credentials.
type AppConfig struct {
	AppID      int64
	PrivateKey *rsa.PrivateKey
}

// InstallationToken holds a short-lived installation access token.
type InstallationToken struct {
	Token     string
	ExpiresAt time.Time
}

// LoadAppConfig reads the GitHub App configuration from environment variables.
// GARY_GITHUB_APP_ID: the App ID (numeric)
// GARY_GITHUB_APP_PRIVATE_KEY: path to PEM file or inline PEM content
func LoadAppConfig() (*AppConfig, error) {
	appIDStr := os.Getenv("GARY_GITHUB_APP_ID")
	if appIDStr == "" {
		return nil, fmt.Errorf("GARY_GITHUB_APP_ID not set")
	}

	var appID int64
	if _, err := fmt.Sscanf(appIDStr, "%d", &appID); err != nil {
		return nil, fmt.Errorf("invalid GARY_GITHUB_APP_ID: %w", err)
	}

	keyPath := os.Getenv("GARY_GITHUB_APP_PRIVATE_KEY")
	if keyPath == "" {
		return nil, fmt.Errorf("GARY_GITHUB_APP_PRIVATE_KEY not set")
	}

	var pemData []byte
	// Try as file path first
	if data, err := os.ReadFile(keyPath); err == nil {
		pemData = data
	} else {
		// Try as inline PEM content
		pemData = []byte(keyPath)
	}

	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS1
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
	}

	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not RSA")
	}

	return &AppConfig{
		AppID:      appID,
		PrivateKey: rsaKey,
	}, nil
}

// GenerateJWT creates a signed JWT for GitHub App authentication.
func (c *AppConfig) GenerateJWT() (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"iat": now.Unix() - 60,       // Issued at (60s clock drift)
		"exp": now.Unix() + (10 * 60), // Expires in 10 minutes
		"iss": c.AppID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(c.PrivateKey)
}

// GetInstallationToken exchanges a JWT for an installation access token.
// installationID is the GitHub App installation ID for the target org/repo.
func (c *AppConfig) GetInstallationToken(installationID int64) (*InstallationToken, error) {
	jwtToken, err := c.GenerateJWT()
	if err != nil {
		return nil, fmt.Errorf("generate JWT: %w", err)
	}

	// POST to GitHub API
	url := fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", installationID)

	req, err := createRequest("POST", url, jwtToken)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := executeRequest(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}

	return &InstallationToken{
		Token:     resp.Token,
		ExpiresAt: resp.ExpiresAt,
	}, nil
}

// TokenResponse is the GitHub API response for installation token.
type TokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// IsExpired reports whether the token has expired.
func (t *InstallationToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

func createRequest(method, url, token string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	return req, nil
}

func executeRequest(req *http.Request) (*TokenResponse, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("GitHub API %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &tokenResp, nil
}
