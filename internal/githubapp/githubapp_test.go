package githubapp

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testPrivateKeyPEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
}

func TestParseRepositoryTrustBoundary(t *testing.T) {
	for _, raw := range []string{
		"https://github.com/matthewjameswatkins1978-cyber/gary.git",
		"https://github.com/matthewjameswatkins1978-cyber/judgement-call.git",
		"https://github.com/matthewjameswatkins1978-cyber/book-burner.git",
		"https://github.com/matthewjameswatkins1978-cyber/a-new-private-repo.git",
	} {
		if _, err := ParseRepository(raw); err != nil {
			t.Errorf("allowed repository %q rejected: %v", raw, err)
		}
	}

	for _, raw := range []string{
		"https://github.com/another-owner/book-burner.git",
		"https://github.com/matthewjameswatkins1978-cyber/book-burner.git.evil",
		"https://github.com.evil/matthewjameswatkins1978-cyber/book-burner.git",
		"https://github.com/matthewjameswatkins1978-cyber/book-burner",
	} {
		if _, err := ParseRepository(raw); err == nil {
			t.Errorf("disallowed repository %q was accepted", raw)
		}
	}
}

func TestProviderMintsAndCachesPerRepositoryScope(t *testing.T) {
	keyPEM := testPrivateKeyPEM(t)
	requests := 0
	var scopes []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodPost || r.URL.Path != "/app/installations/123/access_tokens" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body struct {
			Repositories []string `json:"repositories"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode token request: %v", err)
		}
		if len(body.Repositories) != 1 {
			t.Fatalf("token scope = %#v, want exactly one repository", body.Repositories)
		}
		scopes = append(scopes, body.Repositories[0])
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"token":"fresh-` + body.Repositories[0] + `","expires_at":"2099-01-01T00:00:00Z"}`))
	}))
	defer server.Close()

	provider, err := NewProvider("4717810", "123", keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	provider.apiURL = server.URL
	provider.client = server.Client()
	bookBurner, _ := ParseRepository("https://github.com/matthewjameswatkins1978-cyber/book-burner.git")
	gary, _ := ParseRepository("https://github.com/matthewjameswatkins1978-cyber/gary.git")

	first, err := provider.TokenForRepository(bookBurner)
	if err != nil || first != "fresh-book-burner" {
		t.Fatalf("first token = %q, %v", first, err)
	}
	second, err := provider.TokenForRepository(bookBurner)
	if err != nil || second != first {
		t.Fatalf("cached token = %q, %v", second, err)
	}
	if _, err := provider.TokenForRepository(gary); err != nil {
		t.Fatal(err)
	}
	if requests != 2 || len(scopes) != 2 || scopes[0] != "book-burner" || scopes[1] != "gary" {
		t.Fatalf("requests=%d scopes=%v, want one request each for book-burner and gary", requests, scopes)
	}

	provider.tokens[bookBurner.FullName()] = cachedToken{token: "near-expiry", expires: time.Now().Add(time.Minute)}
	if _, err := provider.TokenForRepository(bookBurner); err != nil {
		t.Fatal(err)
	}
	if requests != 3 {
		t.Fatalf("near-expiry token was not renewed: requests = %d", requests)
	}
}

func TestTokenFromEnvForRepositoryFailsClosedWhenInstallationIsMissing(t *testing.T) {
	t.Setenv("GARY_GITHUB_APP_ID", "4717810")
	t.Setenv("GARY_GITHUB_APP_INSTALLATION_ID", "")
	t.Setenv("GARY_GITHUB_APP_PRIVATE_KEY", testPrivateKeyPEM(t))
	t.Setenv("GARY_GITHUB_TOKEN", "reusable-token-must-not-be-used")

	_, err := TokenFromEnvForRepository("https://github.com/matthewjameswatkins1978-cyber/book-burner.git")
	if err == nil {
		t.Fatal("missing App installation should fail closed")
	}
}

func TestProviderFailsClosedWhenGitHubInstallationIsUnavailable(t *testing.T) {
	provider, err := NewProvider("4717810", "missing-installation", testPrivateKeyPEM(t))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	provider.apiURL = server.URL
	provider.client = server.Client()

	repo, _ := ParseRepository("https://github.com/matthewjameswatkins1978-cyber/book-burner.git")
	if _, err := provider.TokenForRepository(repo); err == nil {
		t.Fatal("unavailable App installation should fail closed")
	}
}

func TestTokenFromEnvForRepositoryRejectsAmbientTokenWithoutAppConfig(t *testing.T) {
	for _, name := range []string{"GARY_GITHUB_APP_ID", "GARY_GITHUB_APP_INSTALLATION_ID", "GARY_GITHUB_APP_PRIVATE_KEY"} {
		t.Setenv(name, "")
	}
	t.Setenv("GARY_GITHUB_TOKEN", "local-token")

	if _, err := TokenFromEnvForRepository("https://github.com/matthewjameswatkins1978-cyber/judgement-call.git"); err == nil {
		t.Fatal("ambient token should not be accepted as worker credentials")
	}
}

func TestNewProviderRejectsNonRSAKey(t *testing.T) {
	if _, err := NewProvider("1", "2", "not-a-key"); err == nil {
		t.Fatal("expected invalid PEM to be rejected")
	}
}
