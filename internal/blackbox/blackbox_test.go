package blackbox

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixtureStore(t *testing.T) *Store {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "jobs"), 0o700); err != nil {
		t.Fatal(err)
	}
	state := map[string]any{
		"version": 2, "status": "FAILED", "created_at": "2026-09-14T10:00:00Z", "updated_at": "2026-09-14T10:05:00Z", "matthew_interventions": 1,
		"job":     map[string]any{"id": "job-one", "repo_path": "C:\\work\\tethers", "base_ref": "master", "task": "Repair worker recovery\nwith detail", "acceptance": []string{"go test ./..."}},
		"workers": []map[string]any{{"id": "worker-1", "profile": "gpt-5.6-luna", "status": "ACCEPTANCE_FAILED", "worktree": "C:\\worktrees\\one", "branch": "gary/job-one", "base_sha": "abc", "commit_sha": "def", "log_path": "C:\\logs\\one.jsonl"}},
	}
	data, _ := json.MarshalIndent(state, "", "  ")
	if err := os.WriteFile(filepath.Join(root, "jobs", "job-one.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	events := strings.Join([]string{
		`{"event_id":"created","job_id":"job-one","type":"JOB_CREATED","timestamp":"2026-09-14T10:00:00Z"}`,
		`{"event_id":"exit","job_id":"job-one","worker_id":"worker-1","type":"WORKER_EXITED","timestamp":"2026-09-14T10:03:00Z","data":{"exit_code":1}}`,
		`{"event_id":"accept","job_id":"job-one","worker_id":"worker-1","type":"ACCEPTANCE_COMPLETED","timestamp":"2026-09-14T10:04:00Z","data":{"passed":false,"candidate_present":true}}`,
		`{"event_id":"failed","job_id":"job-one","type":"JOB_FAILED","timestamp":"2026-09-14T10:05:00Z"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(root, "jobs", "job-one.events.jsonl"), []byte(events), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func TestRebuildAndNoteAreEvidenceDerived(t *testing.T) {
	store := fixtureStore(t)
	records, err := store.Rebuild()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].FailureCategories[0] != "acceptance-failed" {
		t.Fatalf("unexpected records: %+v", records)
	}
	if records[0].NextAction == "" || records[0].LastAction == "" {
		t.Fatalf("missing recovery breadcrumb: %+v", records[0])
	}
	note, err := store.RegenerateNote("job-one")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(note)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "What went wrong") || !strings.Contains(string(data), "acceptance-failed") {
		t.Fatalf("note is not useful: %s", data)
	}
	before, _ := os.ReadFile(filepath.Join(store.Root, "jobs", "job-one.events.jsonl"))
	if _, err := store.AppendJobEvent("job-one", "OPERATOR_NOTE", map[string]any{"text": "retry after api_key=secret"}); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(store.Root, "jobs", "job-one.events.jsonl"))
	if !strings.HasPrefix(string(after), string(before)) || !strings.Contains(string(after), "[REDACTED]") {
		t.Fatalf("journal was not append-only/redacted: %s", after)
	}
}

func TestHistoryRebuildsAfterCorruptIndexAndFilters(t *testing.T) {
	store := fixtureStore(t)
	if _, err := store.Rebuild(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.historyPath(), []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	records, err := store.Records()
	if err != nil || len(records) != 1 {
		t.Fatalf("corrupt index was not rebuildable: %v %+v", err, records)
	}
	cutoff, err := ParseSince("7d", time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC))
	if err != nil || cutoff.Format(time.RFC3339) != "2026-09-07T12:00:00Z" {
		t.Fatalf("since parse: %v %s", err, cutoff)
	}
	if _, err := ParseSince("nonsense", time.Now()); err == nil {
		t.Fatal("invalid since filter accepted")
	}
}

func TestRelationsIncidentsAndCloseoutProof(t *testing.T) {
	store := fixtureStore(t)
	state := filepath.Join(store.Root, "jobs", "job-two.json")
	data := []byte(`{"status":"DONE","created_at":"2026-09-14T11:00:00Z","updated_at":"2026-09-14T11:01:00Z","job":{"id":"job-two","repo_path":"C:\\work\\tethers","base_ref":"master","task":"verify","acceptance":["go test ./..."]}}`)
	if err := os.WriteFile(state, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddRelationship("job-one", "job-two", "repair", "explicit retry", "INC-EXAMPLE"); err != nil {
		t.Fatal(err)
	}
	inc, err := store.OpenIncident(Incident{Class: "worker-stall", JobIDs: []string{"job-one"}, Symptom: "worker disappeared", Expected: "worker exits or requests Matthew", SuspectedCause: "control path may have died"})
	if err != nil {
		t.Fatal(err)
	}
	if inc.SuspectedCause == "" || inc.ProvenCause != "" {
		t.Fatalf("cause semantics collapsed: %+v", inc)
	}
	if _, err := store.CloseIncident(inc.ID, "worker process was gone while state remained RUNNING", "reconcile before retry", "go test ./internal/yard -run TestRecovery"); err != nil {
		t.Fatal(err)
	}
	all, err := store.Incidents()
	if err != nil || len(all) != 1 || all[0].Status != "RESOLVED" || all[0].RegressionProof == "" {
		t.Fatalf("closeout projection: %v %+v", err, all)
	}
	if _, err := store.CloseIncident("missing", "cause", "fix", "proof"); err == nil {
		t.Fatal("unknown incident accepted")
	}
}

func TestBundleRedactsSecretsAndKeepsReadableState(t *testing.T) {
	store := fixtureStore(t)
	packetDir := filepath.Join(store.Root, "jobs", "job-one")
	if err := os.MkdirAll(packetDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packetDir, "ORIGINAL_JOB.md"), []byte("do work with api_key=super-secret\n"+strings.Repeat("evidence ", 9000)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AppendJobEvent("job-one", "TOOL_CONFIG_CHANGED", map[string]any{"api_key": "super-secret", "detail": "safe"}); err != nil {
		t.Fatal(err)
	}
	bundle, err := store.DiagnosticBundle("job-one", 5)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(bundle)
	if strings.Contains(string(data), "super-secret") || !strings.Contains(string(data), "[REDACTED]") {
		t.Fatalf("bundle leaked secret: %s", data)
	}
	if !bundle.PacketTruncated || len(bundle.Packet) > 64*1024+64 {
		t.Fatalf("large packet was not bounded: truncated=%v bytes=%d", bundle.PacketTruncated, len(bundle.Packet))
	}
	if bundle.NextAction == "" || len(bundle.Events) == 0 {
		t.Fatalf("bundle lost recovery evidence: %+v", bundle)
	}
}
