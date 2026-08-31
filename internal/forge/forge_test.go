package forge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSupportedFamiliesHaveDeterministicPlans(t *testing.T) {
	for _, family := range SupportedFamilies() {
		plan, err := BuildPlan(Request{Family: family, Name: "demo", Module: "example/demo"})
		if err != nil || plan.Family != family || len(plan.Files) == 0 {
			t.Fatalf("family %s plan = %+v err=%v", family, plan, err)
		}
	}
}

func TestForgeDoesNotOverwriteExistingFiles(t *testing.T) {
	root := t.TempDir()
	readme := filepath.Join(root, "README.md")
	if err := os.WriteFile(readme, []byte("operator content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Bootstrap(root, Request{Family: Generic, Name: "demo"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(readme)
	if err != nil || string(data) != "operator content" {
		t.Fatalf("Forge overwrote existing README: %q err=%v", data, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		t.Fatalf("generic Forge did not initialize repository: %v", err)
	}
}
