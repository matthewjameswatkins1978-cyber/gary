package pusher

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadAppConfig(t *testing.T) {
	// Generate a test RSA key
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	// Write PEM to file
	pemData := encodePrivateKey(key)
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "test-key.pem")
	if err := os.WriteFile(keyPath, pemData, 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	// Set env vars
	t.Setenv("GARY_GITHUB_APP_ID", "12345")
	t.Setenv("GARY_GITHUB_APP_PRIVATE_KEY", keyPath)

	cfg, err := LoadAppConfig()
	if err != nil {
		t.Fatalf("LoadAppConfig: %v", err)
	}

	if cfg.AppID != 12345 {
		t.Errorf("AppID = %d, want 12345", cfg.AppID)
	}
}

func TestGenerateJWT(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	cfg := &AppConfig{
		AppID:      12345,
		PrivateKey: key,
	}

	token, err := cfg.GenerateJWT()
	if err != nil {
		t.Fatalf("GenerateJWT: %v", err)
	}

	if token == "" {
		t.Error("GenerateJWT returned empty token")
	}
}

func TestInstallationTokenNotExpired(t *testing.T) {
	token := &InstallationToken{
		Token:     "test-token",
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}

	if token.IsExpired() {
		t.Error("token should not be expired")
	}
}

func TestInstallationTokenExpired(t *testing.T) {
	token := &InstallationToken{
		Token:     "test-token",
		ExpiresAt: time.Now().Add(-10 * time.Minute),
	}

	if !token.IsExpired() {
		t.Error("token should be expired")
	}
}

func TestLoadAppConfigMissingAppID(t *testing.T) {
	t.Setenv("GARY_GITHUB_APP_ID", "")
	t.Setenv("GARY_GITHUB_APP_PRIVATE_KEY", "")

	_, err := LoadAppConfig()
	if err == nil {
		t.Error("expected error for missing app ID")
	}
}

func TestLoadAppConfigInvalidPEM(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "bad-key.pem")
	os.WriteFile(keyPath, []byte("not a pem"), 0o600)

	t.Setenv("GARY_GITHUB_APP_ID", "12345")
	t.Setenv("GARY_GITHUB_APP_PRIVATE_KEY", keyPath)

	_, err := LoadAppConfig()
	if err == nil {
		t.Error("expected error for invalid PEM")
	}
}

func encodePrivateKey(key *rsa.PrivateKey) []byte {
	privKeyBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		panic(err)
	}
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privKeyBytes,
	})
}
