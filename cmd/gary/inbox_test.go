package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/jobs"
)

func TestReadInboxValidatesAndSkipsAcceptedJob(t *testing.T) {
	pending := filepath.Join(t.TempDir(), "gary", "inbox", "pending")
	if err := os.MkdirAll(pending, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"id":"smoke-inbox","repo":"https://github.com/matthewjameswatkins1978-cyber/gary.git","base_branch":"master","task":"Run the harmless inbox smoke check only.","acceptance":["go test ./..."]}`
	if err := os.WriteFile(filepath.Join(pending, "smoke-inbox.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := readInbox(pending)
	if err != nil {
		t.Fatalf("readInbox: %v", err)
	}
	if len(entries) != 1 || entries[0].job.ID != "smoke-inbox" {
		t.Fatalf("unexpected entries: %#v", entries)
	}
	if err := writeInboxStatus(pending, entries[0], "accepted", "test"); err != nil {
		t.Fatalf("writeInboxStatus: %v", err)
	}
	result := jobs.RunResult{BatchJobID: "batch-1", Branch: "gary/smoke", CommitSHA: "abc123"}
	if err := writeInboxResult(pending, entries[0], "accepted", "verified", result); err != nil {
		t.Fatalf("writeInboxResult: %v", err)
	}
	entries, err = readInbox(pending)
	if err != nil || entries[0].status != "accepted" {
		t.Fatalf("accepted status not preserved: %#v, %v", entries, err)
	}
	status, err := readInboxStatus(pending, "smoke-inbox")
	if err != nil || status.BatchJobID != "batch-1" || status.CommitSHA != "abc123" {
		t.Fatalf("result metadata not preserved: %#v, %v", status, err)
	}
}

func TestValidateInboxJobRejectsUnsafeManifest(t *testing.T) {
	job := &jobs.Job{
		ID:         "Bad_ID",
		Repo:       "https://github.com/other/repo.git",
		BaseBranch: "master",
		Task:       "x",
		Acceptance: []string{"go test ./...; curl example.com"},
	}
	if err := validateInboxJob(job); err == nil {
		t.Fatal("expected unsafe manifest to be rejected")
	}
}

func TestInboxResultState(t *testing.T) {
	for _, test := range []struct{ in, want string }{
		{"DONE", "accepted"},
		{"BLOCKED", "blocked"},
		{"FAILED", "failed"},
	} {
		got, _ := inboxResultState(test.in)
		if got != test.want {
			t.Errorf("inboxResultState(%q) = %q, want %q", test.in, got, test.want)
		}
	}
}
