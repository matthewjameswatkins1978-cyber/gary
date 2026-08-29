package githubapp

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProviderMintsAndCachesUntilNearExpiry(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodPost || r.URL.Path != "/app/installations/123/access_tokens" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"token":"fresh-token","expires_at":"2099-01-01T00:00:00Z"}`))
	}))
	defer server.Close()

	provider, err := NewProvider("4717810", "123", string(keyPEM))
	if err != nil {
		t.Fatal(err)
	}
	provider.apiURL = server.URL
	provider.client = server.Client()

	first, err := provider.Token()
	if err != nil || first != "fresh-token" {
		t.Fatalf("first token = %q, %v", first, err)
	}
	second, err := provider.Token()
	if err != nil || second != first {
		t.Fatalf("cached token = %q, %v", second, err)
	}
	if requests != 1 {
		t.Fatalf("token endpoint requests = %d, want 1", requests)
	}

	provider.expires = time.Now().Add(1 * time.Minute)
	if _, err := provider.Token(); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("near-expiry token was not renewed: requests = %d", requests)
	}
}

func TestNewProviderRejectsNonRSAKey(t *testing.T) {
	if _, err := NewProvider("1", "2", "not-a-key"); err == nil {
		t.Fatal("expected invalid PEM to be rejected")
	}
}
