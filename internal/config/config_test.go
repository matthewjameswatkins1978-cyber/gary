package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	os.Unsetenv("GARY_GCP_PROJECT")
	os.Unsetenv("GARY_GCP_REGION")
	os.Unsetenv("GARY_GCP_ZONE")
	os.Unsetenv("GARY_GITHUB_REPO")
	os.Unsetenv("GARY_RESULTS_DIR")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ResultsDir != "results" && cfg.ResultsDir != "./results" && cfg.ResultsDir != ".\\results" {
		t.Errorf("default ResultsDir = %q, want results, ./results, or .\\results", cfg.ResultsDir)
	}
	if cfg.GCPProject != "" {
		t.Errorf("GCPProject = %q, want empty", cfg.GCPProject)
	}
}

func TestLoadFromEnv(t *testing.T) {
	os.Setenv("GARY_GCP_PROJECT", "my-project")
	os.Setenv("GARY_GCP_REGION", "us-central1")
	os.Setenv("GARY_GCP_ZONE", "us-central1-a")
	os.Setenv("GARY_GITHUB_REPO", "owner/repo")
	os.Setenv("GARY_RESULTS_DIR", "/tmp/results")
	defer func() {
		os.Unsetenv("GARY_GCP_PROJECT")
		os.Unsetenv("GARY_GCP_REGION")
		os.Unsetenv("GARY_GCP_ZONE")
		os.Unsetenv("GARY_GITHUB_REPO")
		os.Unsetenv("GARY_RESULTS_DIR")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.GCPProject != "my-project" {
		t.Errorf("GCPProject = %q, want my-project", cfg.GCPProject)
	}
	if cfg.GCPRegion != "us-central1" {
		t.Errorf("GCPRegion = %q, want us-central1", cfg.GCPRegion)
	}
	if cfg.GCPZone != "us-central1-a" {
		t.Errorf("GCPZone = %q, want us-central1-a", cfg.GCPZone)
	}
	if cfg.GitHubRepo != "owner/repo" {
		t.Errorf("GitHubRepo = %q, want owner/repo", cfg.GitHubRepo)
	}
	if cfg.ResultsDir != "/tmp/results" {
		t.Errorf("ResultsDir = %q, want /tmp/results", cfg.ResultsDir)
	}
}

func TestValidateMissing(t *testing.T) {
	cfg := &Config{}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing config, got nil")
	}
}

func TestValidatePartial(t *testing.T) {
	cfg := &Config{GCPProject: "p"}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing region, got nil")
	}

	cfg = &Config{GCPRegion: "r"}
	err = cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing project, got nil")
	}
}

func TestValidateComplete(t *testing.T) {
	cfg := &Config{GCPProject: "p", GCPRegion: "r"}
	err := cfg.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHasGCPConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
		want bool
	}{
		{"both set", &Config{GCPProject: "p", GCPRegion: "r"}, true},
		{"project only", &Config{GCPProject: "p"}, false},
		{"region only", &Config{GCPRegion: "r"}, false},
		{"neither", &Config{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.HasGCPConfig(); got != tt.want {
				t.Errorf("HasGCPConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadImplementerModelDefault(t *testing.T) {
	os.Unsetenv("GARY_IMPLEMENTER_MODEL")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ImplementerModel != "gemini-3.5-flash-lite" {
		t.Errorf("ImplementerModel = %q, want gemini-3.5-flash-lite", cfg.ImplementerModel)
	}
}

func TestLoadImplementerModelFromEnv(t *testing.T) {
	os.Setenv("GARY_IMPLEMENTER_MODEL", "gemini-2.5-pro")
	defer os.Unsetenv("GARY_IMPLEMENTER_MODEL")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ImplementerModel != "gemini-2.5-pro" {
		t.Errorf("ImplementerModel = %q, want gemini-2.5-pro", cfg.ImplementerModel)
	}
}

func TestLoadMasterModelDefault(t *testing.T) {
	os.Unsetenv("GARY_MASTER_MODEL")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MasterModel != "gemini-3.8-flash" {
		t.Errorf("MasterModel = %q, want gemini-3.8-flash", cfg.MasterModel)
	}
}

func TestLoadGeminiAPIKey(t *testing.T) {
	os.Setenv("GEMINI_API_KEY", "test-key-123")
	defer os.Unsetenv("GEMINI_API_KEY")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.GeminiAPIKey != "test-key-123" {
		t.Errorf("GeminiAPIKey = %q, want test-key-123", cfg.GeminiAPIKey)
	}
}

func TestLoadWorkerRuntimeDefault(t *testing.T) {
	os.Unsetenv("GARY_WORKER_RUNTIME")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.WorkerRuntime != "batch" {
		t.Errorf("WorkerRuntime = %q, want batch", cfg.WorkerRuntime)
	}
}

func TestLoadWorkerRuntimeFromEnv(t *testing.T) {
	os.Setenv("GARY_WORKER_RUNTIME", "managed")
	defer os.Unsetenv("GARY_WORKER_RUNTIME")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.WorkerRuntime != "managed" {
		t.Errorf("WorkerRuntime = %q, want managed", cfg.WorkerRuntime)
	}
}
