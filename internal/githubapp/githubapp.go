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
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultAPIURL = "https://api.github.com"
	refreshBefore = 5 * time.Minute
	TrustedOwner  = "matthewjameswatkins1978-cyber"
)

// Repository is the canonical identity of a trusted GitHub repository.
// Repository authorization is account-scoped, while installation tokens are
// still minted for exactly one repository at a time.
type Repository struct {
	Owner string
	Name  string
}

func (r Repository) FullName() string { return r.Owner + "/" + r.Name }

// ParseRepository accepts only HTTPS GitHub repositories under the approved
// GARY owner. The repository name is intentionally not allowlisted: adding a
// new private repository under the trusted owner must not require a source
// change.
func ParseRepository(raw string) (Repository, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || !strings.EqualFold(u.Hostname(), "github.com") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return Repository{}, errors.New("repo must be an HTTPS GitHub repository")
	}
	path := u.EscapedPath()
	if path == "" || path != u.Path || !strings.HasPrefix(path, "/") || strings.HasSuffix(path, "/") {
		return Repository{}, errors.New("repo must use the canonical GitHub URL")
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) != 2 || !strings.HasSuffix(parts[1], ".git") {
		return Repository{}, errors.New("repo must use owner/name.git")
	}
	owner := parts[0]
	name := strings.TrimSuffix(parts[1], ".git")
	if !strings.EqualFold(owner, TrustedOwner) || !validRepositoryName(name) {
		return Repository{}, fmt.Errorf("repo is outside the approved GitHub owner %q or has an invalid name", TrustedOwner)
	}
	return Repository{Owner: TrustedOwner, Name: name}, nil
}

func validRepositoryName(name string) bool {
	if name == "" || len(name) > 100 {
		return false
	}
	for _, r := range name {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
			return false
		}
	}
	return true
}

var (
	cacheMu        sync.Mutex
	cachedKey      string
	cachedProvider *Provider
)

// Provider mints and caches one repository-scoped installation token per
// repository until it is close to expiry.
type Provider struct {
	appID          string
	installationID string
	privateKey     *rsa.PrivateKey
	apiURL         string
	client         *http.Client

	mu     sync.Mutex
	tokens map[string]cachedToken
}

type cachedToken struct {
	token   string
	expires time.Time
}

// TokenFromEnvForRepository returns a short-lived token restricted to repoURL.
// Public repositories may run without a token when no App configuration is
// present. Ambient tokens are rejected so a reusable PAT cannot reach a
// worker; configured App credentials are mandatory for private operations.
func TokenFromEnvForRepository(repoURL string) (string, error) {
	repo, err := ParseRepository(repoURL)
	if err != nil {
		return "", err
	}
	appID := strings.TrimSpace(os.Getenv("GARY_GITHUB_APP_ID"))
	installationID := strings.TrimSpace(os.Getenv("GARY_GITHUB_APP_INSTALLATION_ID"))
	privateKey := os.Getenv("GARY_GITHUB_APP_PRIVATE_KEY")
	if appID == "" && installationID == "" && privateKey == "" {
		if strings.TrimSpace(os.Getenv("GARY_GITHUB_TOKEN")) != "" {
			return "", errors.New("ambient GitHub token is not accepted; configure the GARY GitHub App")
		}
		return "", nil
	}
	if err := ValidateAppConfiguration(); err != nil {
		return "", err
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
	return provider.TokenForRepository(repo)
}

// ValidateAppConfiguration checks the control-plane-only App credentials
// without contacting GitHub. Token minting still verifies the installation at
// the API boundary and returns an error if it is missing or unauthorized.
func ValidateAppConfiguration() error {
	appID := strings.TrimSpace(os.Getenv("GARY_GITHUB_APP_ID"))
	installationID := strings.TrimSpace(os.Getenv("GARY_GITHUB_APP_INSTALLATION_ID"))
	privateKey := os.Getenv("GARY_GITHUB_APP_PRIVATE_KEY")
	if appID == "" || installationID == "" || strings.TrimSpace(privateKey) == "" {
		return errors.New("incomplete GitHub App credentials")
	}
	if _, err := strconv.ParseInt(installationID, 10, 64); err != nil {
		return fmt.Errorf("invalid GitHub App installation ID: %w", err)
	}
	return nil
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
		tokens:         make(map[string]cachedToken),
	}, nil
}

// TokenForRepository returns a cached token or mints a fresh token restricted
// to exactly repo. It never creates an installation-wide token.
func (p *Provider) TokenForRepository(repo Repository) (string, error) {
	if repo.Owner != TrustedOwner || !validRepositoryName(repo.Name) {
		return "", errors.New("repository is outside the approved GitHub trust boundary")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if cached, ok := p.tokens[repo.FullName()]; ok && cached.token != "" && time.Until(cached.expires) > refreshBefore {
		return cached.token, nil
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
	body, err := json.Marshal(map[string][]string{"repositories": []string{repo.Name}})
	if err != nil {
		return "", fmt.Errorf("encode GitHub App token request: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return "", fmt.Errorf("create GitHub App token request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")
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
	p.tokens[repo.FullName()] = cachedToken{token: payload.Token, expires: expires}
	return payload.Token, nil
}
