package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
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
	if got.Task != want || !bytes.Equal(got.PacketBytes, []byte(want)) {
		t.Fatalf("whole packet changed during intake\ngot: %q\nwant:%q", got.Task, want)
	}
	if got.PacketDigest == "" || got.PacketSource != packet {
		t.Fatalf("packet provenance missing: digest=%q source=%q", got.PacketDigest, got.PacketSource)
	}
	if got.MaxRunSeconds != 3600 {
		t.Fatalf("default max_run_seconds=%d want 3600", got.MaxRunSeconds)
	}
}

func TestPersistWorkPacketIsExactAndImmutable(t *testing.T) {
	dir := t.TempDir()
	packet := []byte("a\r\nb\n")
	digest := sha256.Sum256(packet)
	m := &workManifest{ID: "packet-test", PacketBytes: packet, PacketDigest: fmt.Sprintf("%x", digest[:]), PacketSource: "JOB.md"}
	if err := persistWorkPacket(dir, m); err != nil {
		t.Fatal(err)
	}
	storedDir := filepath.Join(dir, "packets", "packet-test-"+m.PacketDigest[:12])
	got, err := os.ReadFile(filepath.Join(storedDir, "packet.raw"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, m.PacketBytes) {
		t.Fatalf("stored packet changed bytes: %q", got)
	}
	var p workPacketProvenance
	data, err := os.ReadFile(filepath.Join(storedDir, "provenance.json"))
	if err != nil || json.Unmarshal(data, &p) != nil {
		t.Fatalf("bad provenance: %v", err)
	}
	if p.SHA256 != m.PacketDigest || p.SizeBytes != len(m.PacketBytes) {
		t.Fatalf("bad provenance: %+v", p)
	}
	if err := persistWorkPacket(dir, m); err != nil {
		t.Fatalf("identical packet should be idempotent: %v", err)
	}
}

func TestPersistWorkPacketRejectsMismatchedDigest(t *testing.T) {
	err := persistWorkPacket(t.TempDir(), &workManifest{
		ID: "packet-test", PacketBytes: []byte("a\r\nb\n"), PacketDigest: "not-the-byte-digest", PacketSource: "JOB.md",
	})
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected digest mismatch rejection, got %v", err)
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
	if got == "" || len(got) > 18 {
		t.Fatalf("bad bounded id %q", got)
	}
	for _, r := range got {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			t.Fatalf("unsafe character %q in %q", r, got)
		}
	}
}
