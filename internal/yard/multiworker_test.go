package yard

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSpawnWorkerForYardmasterRejectsDistinctUnmarkedSecondTask(t *testing.T) {
	root := t.TempDir()
	m, err := NewManager(JobSpec{ID: "strict-second", RepoPath: root, BaseRef: "main", Task: "test", Acceptance: []string{"exit 0"}}, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	running := &WorkerRecord{ID: "one", JobID: "strict-second", Profile: "gpt-5.6-luna", Task: "first task", Status: "RUNNING", PID: os.Getpid(), Worktree: root}
	m.state.Workers = []*WorkerRecord{running}
	m.state.Worker = running
	if err := m.saveLocked(); err != nil {
		t.Fatal(err)
	}

	if _, err := m.SpawnWorkerForYardmaster("different task", "gpt-5.6-luna", false); err == nil || !strings.Contains(err.Error(), "declared independent") {
		t.Fatalf("distinct second task was not rejected: %v", err)
	}
	view, err := m.SpawnWorkerForYardmaster("first task", "gpt-5.6-luna", false)
	if err != nil || view.Record.ID != "one" {
		t.Fatalf("same-task retry should be idempotent: view=%+v err=%v", view, err)
	}
}

func TestProgressRequiresWholeWorkerSetToBeIntegrated(t *testing.T) {
	root := t.TempDir()
	m, err := NewManager(JobSpec{ID: "progress", RepoPath: root, BaseRef: "main", Task: "test", Acceptance: []string{"exit 0"}}, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	m.state.Workers = []*WorkerRecord{
		{ID: "one", Status: "INTEGRATED", Integrated: true},
		{ID: "two", Status: "ACCEPTED"},
	}
	m.state.Worker = m.state.Workers[1]
	progress := m.Progress()
	if progress.AllIntegrated || progress.Total != 2 || progress.Integrated != 1 || progress.Pending != 1 {
		t.Fatalf("unexpected partial progress: %+v", progress)
	}
	m.state.Workers[1].Status = "INTEGRATED"
	m.state.Workers[1].Integrated = true
	progress = m.Progress()
	if !progress.AllIntegrated || progress.Integrated != 2 {
		t.Fatalf("whole worker set was not recognized as integrated: %+v", progress)
	}
}

func TestWaitForAnyWorkerEventDoesNotTrustLegacyCurrentWorker(t *testing.T) {
	root := t.TempDir()
	m, err := NewManager(JobSpec{ID: "wait-all", RepoPath: root, BaseRef: "main", Task: "test", Acceptance: []string{"exit 0"}}, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	alive := &WorkerRecord{ID: "alive", Status: "RUNNING", PID: os.Getpid(), Worktree: root}
	completed := &WorkerRecord{ID: "completed", Status: "COMPLETED", PID: -1, Worktree: root}
	m.state.Workers = []*WorkerRecord{alive, completed}
	m.state.Worker = completed

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	if _, err := m.WaitForAnyWorkerEvent(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("wait returned because the legacy current worker was complete while another worker was alive: %v", err)
	}
}

func TestIntegrateWorkerSerialLandsTwoIndependentCandidates(t *testing.T) {
	repo, stateDir, baseSHA := initIntegrationRepo(t)
	workerOne := createWorkerCandidate(t, repo, "worker-one", "one.txt", "one\n")
	workerTwo := createWorkerCandidate(t, repo, "worker-two", "two.txt", "two\n")

	m, err := NewManager(JobSpec{ID: "serial-two", RepoPath: repo, BaseRef: "main", Task: "integrate two", Acceptance: []string{"git diff --check"}}, stateDir)
	if err != nil {
		t.Fatal(err)
	}
	one := &WorkerRecord{ID: "one", JobID: "serial-two", Status: "ACCEPTED", Worktree: workerOne, BaseSHA: baseSHA, CommitSHA: captureGit(workerOne, "rev-parse", "HEAD")}
	two := &WorkerRecord{ID: "two", JobID: "serial-two", Status: "ACCEPTED", Worktree: workerTwo, BaseSHA: baseSHA, CommitSHA: captureGit(workerTwo, "rev-parse", "HEAD")}
	m.state.Workers = []*WorkerRecord{one, two}
	m.state.Worker = two
	if err := m.saveLocked(); err != nil {
		t.Fatal(err)
	}

	if err := m.IntegrateWorkerSerial("one"); err != nil {
		t.Fatalf("first integration: %v", err)
	}
	if err := m.IntegrateWorkerSerial("one"); err != nil {
		t.Fatalf("idempotent integration retry: %v", err)
	}
	firstHead := captureGit(repo, "rev-parse", "HEAD")
	if firstHead == baseSHA {
		t.Fatal("first integration did not advance target HEAD")
	}
	if err := m.IntegrateWorkerSerial("two"); err != nil {
		t.Fatalf("second integration from the original base should land serially: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "one.txt")); err != nil {
		t.Fatalf("first candidate missing after serial integration: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "two.txt")); err != nil {
		t.Fatalf("second candidate missing after serial integration: %v", err)
	}
	progress := m.Progress()
	if !progress.AllIntegrated || progress.Integrated != 2 {
		t.Fatalf("serial integration did not finish the worker set: %+v", progress)
	}
}

func TestIntegrateWorkerSerialRestoresTargetAfterFinalAcceptanceFailure(t *testing.T) {
	repo, stateDir, baseSHA := initIntegrationRepo(t)
	worker := createWorkerCandidate(t, repo, "worker-bad-compose", "candidate.txt", "candidate\n")
	m, err := NewManager(JobSpec{ID: "rollback", RepoPath: repo, BaseRef: "main", Task: "rollback test", Acceptance: []string{"exit 1"}}, stateDir)
	if err != nil {
		t.Fatal(err)
	}
	record := &WorkerRecord{ID: "candidate", JobID: "rollback", Status: "ACCEPTED", Worktree: worker, BaseSHA: baseSHA, CommitSHA: captureGit(worker, "rev-parse", "HEAD")}
	m.state.Workers = []*WorkerRecord{record}
	m.state.Worker = record
	if err := m.saveLocked(); err != nil {
		t.Fatal(err)
	}

	err = m.IntegrateWorkerSerial("candidate")
	if err == nil || !strings.Contains(err.Error(), "target restored") {
		t.Fatalf("expected failed final acceptance with restoration evidence, got %v", err)
	}
	if got := captureGit(repo, "rev-parse", "HEAD"); got != baseSHA {
		t.Fatalf("target HEAD=%s after rollback, want %s", got, baseSHA)
	}
	if dirty := strings.TrimSpace(captureGit(repo, "status", "--porcelain")); dirty != "" {
		t.Fatalf("target remained dirty after rollback: %s", dirty)
	}
	if got := m.State().Workers[0].Status; got != "ACCEPTED" {
		t.Fatalf("candidate status=%q after rolled-back composition, want ACCEPTED", got)
	}
}

func initIntegrationRepo(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runGit(repo, "init", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runGit(repo, "add", "base.txt"); err != nil {
		t.Fatal(err)
	}
	if err := runGit(repo, "-c", "user.name=yard-test", "-c", "user.email=yard-test@local", "commit", "-m", "base"); err != nil {
		t.Fatal(err)
	}
	return repo, filepath.Join(root, "state"), captureGit(repo, "rev-parse", "HEAD")
}

func createWorkerCandidate(t *testing.T, repo, branch, filename, content string) string {
	t.Helper()
	worktree := filepath.Join(filepath.Dir(repo), branch)
	if err := runGit(repo, "worktree", "add", "-b", branch, worktree, "main"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, filename), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runGit(worktree, "add", filename); err != nil {
		t.Fatal(err)
	}
	if err := runGit(worktree, "-c", "user.name=yard-test", "-c", "user.email=yard-test@local", "commit", "-m", "add "+filename); err != nil {
		t.Fatal(err)
	}
	return worktree
}
