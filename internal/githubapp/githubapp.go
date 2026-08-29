// Package githubapp mints short-lived GitHub App installation tokens on demand.
package githubapp

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultAPIURL = "https://api.github.com"
	refreshBefore = 5 * time.Minute
)

var (
	cacheMu        sync.Mutex
	cachedKey      string
	cachedProvider *Provider
)

// Provider mints and caches one installation token until it is close to expiry.
type Provider struct {
	appID          string
	installationID string
	privateKey     *rsa.PrivateKey
	apiURL         string
	client         *http.Client

	mu      sync.Mutex
	token   string
	expires time.Time
}

// TokenFromEnv returns a current installation token. Complete App credentials
// take precedence over GARY_GITHUB_TOKEN so long jobs renew instead of reusing
// the one token created when the workflow started.
func TokenFromEnv() (string, error) {
	appID := strings.TrimSpace(os.Getenv("GARY_GITHUB_APP_ID"))
	installationID := strings.TrimSpace(os.Getenv("GARY_GITHUB_APP_INSTALLATION_ID"))
	privateKey := os.Getenv("GARY_GITHUB_APP_PRIVATE_KEY")
	if appID == "" && installationID == "" && privateKey == "" {
		return os.Getenv("GARY_GITHUB_TOKEN"), nil
	}
	if appID == "" || installationID == "" || strings.TrimSpace(privateKey) == "" {
		return "", errors.New("incomplete GitHub App credentials")
	}
	if _, err := strconv.ParseInt(installationID, 10, 64); err != nil {
		return "", fmt.Errorf("invalid GitHub App installation ID: %w", err)
	}

	key := appID + "\x00" + installationID + "\x00" + privateKey
	cacheMu.Lock()
	if cachedProvider == nil || cachedKey != key {
		provider, err := NewProvider(appID, installationID, privateKey)
		if err != nil {
			cacheMu.Unlock()
			return "", err
		}
		cachedKey = key
		cachedProvider = provider
	}
	provider := cachedProvider
	cacheMu.Unlock()
	return provider.Token()
}

// NewProvider constructs a provider from an App ID, installation ID and PEM.
func NewProvider(appID, installationID, privateKeyPEM string) (*Provider, error) {
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, errors.New("GitHub App private key is not PEM encoded")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		parsed, pkcs8Err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if pkcs8Err != nil {
			return nil, fmt.Errorf("parse GitHub App private key: %w", err)
		}
		var ok bool
		key, ok = parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("GitHub App private key is not RSA")
		}
	}
	return &Provider{
		appID:          appID,
		installationID: installationID,
		privateKey:     key,
		apiURL:         defaultAPIURL,
		client:         http.DefaultClient,
	}, nil
}

// Token returns a cached token or mints a fresh one.
func (p *Provider) Token() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.token != "" && time.Until(p.expires) > refreshBefore {
		return p.token, nil
	}

	now := time.Now()
	claims := fmt.Sprintf(`{"iat":%d,"exp":%d,"iss":"%s"}`, now.Add(-time.Minute).Unix(), now.Add(9*time.Minute).Unix(), p.appID)
	header := `{"alg":"RS256","typ":"JWT"}`
	unsigned := base64.RawURLEncoding.EncodeToString([]byte(header)) + "." + base64.RawURLEncoding.EncodeToString([]byte(claims))
	hash := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, p.privateKey, crypto.SHA256, hash[:])
	if err != nil {
		return "", fmt.Errorf("sign GitHub App JWT: %w", err)
	}
	jwt := unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)

	url := strings.TrimRight(p.apiURL, "/") + "/app/installations/" + p.installationID + "/access_tokens"
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return "", fmt.Errorf("create GitHub App token request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("mint GitHub App installation token: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("mint GitHub App installation token: HTTP %s", resp.Status)
	}
	var payload struct {
		Token     string `json:"token"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode GitHub App token response: %w", err)
	}
	expires, err := time.Parse(time.RFC3339, payload.ExpiresAt)
	if err != nil || payload.Token == "" {
		return "", errors.New("GitHub App token response lacked a valid token or expiry")
	}
	p.token = payload.Token
	p.expires = expires
	return p.token, nil
}
