package yard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStateSurvivesManagerRestart(t *testing.T) {
	root := t.TempDir()
	spec := JobSpec{ID: "restart-test", RepoPath: root, BaseRef: "main", Task: "test persistence", Acceptance: []string{"exit 0"}}
	first, err := NewManager(spec, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	if err := first.SetThreadID("thr_persisted"); err != nil {
		t.Fatal(err)
	}
	if _, err := first.RequestMatthew("approval is required", "a human approval", "the protected action rejected the worker", "approve the action and rerun with --answer"); err != nil {
		t.Fatal(err)
	}
	second, err := NewManager(spec, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	state := second.State()
	if state.ThreadID != "thr_persisted" || state.Status != "NEEDS_MATTHEW" || state.Blocker == nil {
		t.Fatalf("restart lost durable state: %+v", state)
	}
	if err := second.ResolveBlocker("approved for the disposable canary"); err != nil {
		t.Fatal(err)
	}
	if got := second.State().Status; got != "RUNNING" {
		t.Fatalf("resolved status=%q want RUNNING", got)
	}
}

func TestWorkerStatusReconcilesExistingProcessWithoutSpawning(t *testing.T) {
	root := t.TempDir()
	spec := JobSpec{ID: "reconcile-test", RepoPath: root, BaseRef: "main", Task: "test reconciliation", Acceptance: []string{"exit 0"}}
	m, err := NewManager(spec, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	m.state.Worker = &WorkerRecord{ID: "worker-existing", JobID: spec.ID, Profile: "gpt-5.6-luna", Worktree: root, PID: os.Getpid(), Status: "RUNNING", LogPath: filepath.Join(root, "worker.log")}
	if err := m.saveLocked(); err != nil {
		t.Fatal(err)
	}
	view, err := m.WorkerStatus("worker-existing")
	if err != nil {
		t.Fatal(err)
	}
	if !view.ProcessAlive || view.Record.ID != "worker-existing" {
		t.Fatalf("existing process was not reconciled: %+v", view)
	}
	returned, err := m.SpawnWorker("a duplicate must not start", "gpt-5.6-luna")
	if err != nil {
		t.Fatal(err)
	}
	if returned.Record.ID != "worker-existing" {
		t.Fatalf("spawn duplicated existing worker: %+v", returned)
	}
}

func TestAcceptanceIsIndependentOfWorkerClaim(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "accepted.txt")
	if err := os.WriteFile(marker, []byte("real state"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runGit(root, "init", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	if err := runGit(root, "-c", "user.name=yard-test", "-c", "user.email=yard-test@local", "add", "accepted.txt"); err != nil {
		t.Fatal(err)
	}
	if err := runGit(root, "-c", "user.name=yard-test", "-c", "user.email=yard-test@local", "commit", "-m", "initial"); err != nil {
		t.Fatal(err)
	}
	if err := runGit(root, "checkout", "-b", "worker"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte("accepted candidate"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runGit(root, "-c", "user.name=yard-test", "-c", "user.email=yard-test@local", "add", "accepted.txt"); err != nil {
		t.Fatal(err)
	}
	if err := runGit(root, "-c", "user.name=yard-test", "-c", "user.email=yard-test@local", "commit", "-m", "candidate"); err != nil {
		t.Fatal(err)
	}
	spec := JobSpec{ID: "acceptance-test", RepoPath: root, BaseRef: "main", Task: "test acceptance", Acceptance: []string{"if (Test-Path accepted.txt) { exit 0 } else { exit 1 }"}}
	stateDir := t.TempDir()
	m, err := NewManager(spec, stateDir)
	if err != nil {
		t.Fatal(err)
	}
	m.state.Worker = &WorkerRecord{ID: "worker-stopped", JobID: spec.ID, Profile: "gpt-5.6-luna", Worktree: root, PID: -1, Status: "COMPLETED", AcceptancePath: filepath.Join(root, "acceptance.json")}
	if err := m.saveLocked(); err != nil {
		t.Fatal(err)
	}
	result, err := m.RunAcceptance("worker-stopped")
	if err != nil || !result.Passed || !result.CandidatePresent {
		t.Fatalf("acceptance failed: result=%+v err=%v", result, err)
	}
	if m.State().Worker.Status != "ACCEPTED" {
		t.Fatalf("worker status=%q want ACCEPTED", m.State().Worker.Status)
	}
	data, err := os.ReadFile(filepath.Join(root, "acceptance.json"))
	if err != nil || !strings.Contains(string(data), `"passed": true`) {
		t.Fatalf("acceptance evidence missing: %v %s", err, data)
	}
}
