package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	for _, key := range []string{
		"GARY_GCP_PROJECT", "GARY_GCP_REGION", "GARY_GCP_ZONE", "GARY_GITHUB_REPO",
		"GARY_RESULTS_DIR", "GARY_IMPLEMENTER_MODEL", "GARY_MASTER_MODEL", "GARY_CLOSER_MODEL",
		"GARY_WORKER_RUNTIME",
	} {
		os.Unsetenv(key)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ResultsDir != "results" && cfg.ResultsDir != "./results" && cfg.ResultsDir != ".\\results" {
		t.Fatalf("unexpected ResultsDir %q", cfg.ResultsDir)
	}
	if cfg.ImplementerModel != "gemini-3.8-flash" {
		t.Fatalf("ImplementerModel=%q want gemini-3.8-flash", cfg.ImplementerModel)
	}
	if cfg.MasterModel != "" || cfg.CloserModel != "" {
		t.Fatalf("legacy intelligent fallbacks must have no defaults: master=%q closer=%q", cfg.MasterModel, cfg.CloserModel)
	}
	if cfg.WorkerRuntime != "batch" {
		t.Fatalf("WorkerRuntime=%q want batch", cfg.WorkerRuntime)
	}
}

func TestLoadFromEnv(t *testing.T) {
	values := map[string]string{
		"GARY_GCP_PROJECT": "my-project",
		"GARY_GCP_REGION": "us-central1",
		"GARY_GCP_ZONE": "us-central1-a",
		"GARY_GITHUB_REPO": "owner/repo",
		"GARY_RESULTS_DIR": "/tmp/results",
		"GARY_IMPLEMENTER_MODEL": "human-approved-model",
		"GARY_WORKER_RUNTIME": "batch",
	}
	for k, v := range values {
		os.Setenv(k, v)
		defer os.Unsetenv(k)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GCPProject != "my-project" || cfg.GCPRegion != "us-central1" || cfg.GCPZone != "us-central1-a" {
		t.Fatalf("cloud env not loaded: %#v", cfg)
	}
	if cfg.GitHubRepo != "owner/repo" || cfg.ResultsDir != "/tmp/results" {
		t.Fatalf("repository/results env not loaded: %#v", cfg)
	}
	if cfg.ImplementerModel != "human-approved-model" {
		t.Fatalf("explicit model override lost: %q", cfg.ImplementerModel)
	}
}

func TestValidate(t *testing.T) {
	if err := (&Config{}).Validate(); err == nil {
		t.Fatal("missing cloud config should fail")
	}
	if err := (&Config{GCPProject: "p"}).Validate(); err == nil {
		t.Fatal("missing region should fail")
	}
	if err := (&Config{GCPRegion: "r"}).Validate(); err == nil {
		t.Fatal("missing project should fail")
	}
	if err := (&Config{GCPProject: "p", GCPRegion: "r"}).Validate(); err != nil {
		t.Fatalf("complete cloud config failed: %v", err)
	}
}

func TestHasGCPConfig(t *testing.T) {
	for _, tc := range []struct {
		cfg  *Config
		want bool
	}{
		{&Config{GCPProject: "p", GCPRegion: "r"}, true},
		{&Config{GCPProject: "p"}, false},
		{&Config{GCPRegion: "r"}, false},
		{&Config{}, false},
	} {
		if got := tc.cfg.HasGCPConfig(); got != tc.want {
			t.Fatalf("HasGCPConfig=%v want %v", got, tc.want)
		}
	}
}

func TestLoadGeminiAPIKey(t *testing.T) {
	os.Setenv("GEMINI_API_KEY", "test-key-123")
	defer os.Unsetenv("GEMINI_API_KEY")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GeminiAPIKey != "test-key-123" {
		t.Fatalf("GeminiAPIKey=%q", cfg.GeminiAPIKey)
	}
}
