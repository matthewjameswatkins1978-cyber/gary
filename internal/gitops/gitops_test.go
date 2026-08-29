package gitops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCherryPickSuppliesCommitterIdentity(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	if err := runGit(dir, "init", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "value.txt")
	if err := os.WriteFile(path, []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runGit(dir, "add", "value.txt"); err != nil {
		t.Fatal(err)
	}
	if err := runGit(dir, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-m", "base"); err != nil {
		t.Fatal(err)
	}
	if err := runGit(dir, "checkout", "-b", "worker"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("worker\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runGit(dir, "add", "value.txt"); err != nil {
		t.Fatal(err)
	}
	if err := runGit(dir, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-m", "worker"); err != nil {
		t.Fatal(err)
	}
	sha, err := runGitCapture(dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if err := runGit(dir, "checkout", "main"); err != nil {
		t.Fatal(err)
	}
	if err := CherryPick(dir, sha); err != nil {
		t.Fatalf("cherry-pick without ambient git identity failed: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "worker\n" {
		t.Fatalf("unexpected cherry-picked content: %q", got)
	}
}
