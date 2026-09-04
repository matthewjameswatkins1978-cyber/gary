package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadWorkManifestReadsWholePacket(t *testing.T) {
	dir := t.TempDir()
	packet := filepath.Join(dir, "JOB.md")
	manifest := filepath.Join(dir, "job.json")
	want := "# Complete job\nBuild the real thing, end to end.\n"
	if err := os.WriteFile(packet, []byte(want), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte(`{
  "id":"telltail-live",
  "repo":"https://github.com/example/telltail.git",
  "base_branch":"master",
  "packet_path":"JOB.md",
  "acceptance":["go test ./..."]
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := loadWorkManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if got.Task != want {
		t.Fatalf("whole packet changed during intake\ngot: %q\nwant:%q", got.Task, want)
	}
	if got.MaxRunSeconds != 3600 {
		t.Fatalf("default max_run_seconds=%d want 3600", got.MaxRunSeconds)
	}
}

func TestLoadWorkManifestRejectsOrchestrationAmbiguity(t *testing.T) {
	dir := t.TempDir()
	packet := filepath.Join(dir, "JOB.md")
	manifest := filepath.Join(dir, "job.json")
	_ = os.WriteFile(packet, []byte("whole job"), 0o644)
	_ = os.WriteFile(manifest, []byte(`{
  "id":"job",
  "repo":"https://github.com/example/repo.git",
  "base_branch":"master",
  "packet_path":"JOB.md",
  "task":"child-ish duplicate instruction",
  "acceptance":["go test ./..."]
}`), 0o644)
	if _, err := loadWorkManifest(manifest); err == nil || !strings.Contains(err.Error(), "packet_path or task") {
		t.Fatalf("expected packet/task ambiguity rejection, got %v", err)
	}
}

func TestLoadWorkManifestRequiresRealAcceptance(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "job.json")
	_ = os.WriteFile(manifest, []byte(`{
  "id":"job",
  "repo":"https://github.com/example/repo.git",
  "base_branch":"master",
  "task":"finish it"
}`), 0o644)
	if _, err := loadWorkManifest(manifest); err == nil || !strings.Contains(err.Error(), "acceptance") {
		t.Fatalf("expected acceptance error, got %v", err)
	}
}

func TestParseBlockerRequiresStructuredHumanBoundary(t *testing.T) {
	if parseBlocker("") != nil || parseBlocker("{}") != nil || parseBlocker(`{"summary":"missing kind"}`) != nil {
		t.Fatal("empty or malformed blocker must not become a human escalation")
	}
	b := parseBlocker(`{
  "kind":"model_escalation",
  "summary":"assigned model cannot reliably complete the remaining proof",
  "needed":"human approval for a stronger model",
  "evidence":"two distinct approaches reached the same reasoning failure",
  "unblock_action":"approve or reject model escalation"
}`)
	if b == nil || b.Kind != "model_escalation" || b.UnblockAction == "" {
		t.Fatalf("valid escalation blocker not preserved: %#v", b)
	}
}

func TestShortSimpleIDIsBatchSafeAndBounded(t *testing.T) {
	got := shortSimpleID("Telltail: Real Live Fire / R3", 18)
	if got != "telltail-real-live" {
		t.Fatalf("shortSimpleID=%q", got)
	}
	if len(got) > 18 {
		t.Fatalf("id exceeded bound: %d", len(got))
	}
}
