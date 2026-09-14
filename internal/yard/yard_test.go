package yard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkerCommandProfiles(t *testing.T) {
	t.Setenv("MIMO_API_KEY", "test-only-not-a-real-key")
	t.Setenv("GARY_MIMO_BASE_URL", "https://token-plan-ams.xiaomimimo.com/v1")

	args, metadata, err := workerCommand("gpt-5.6-luna")
	if err != nil || strings.Join(args, " ") != "exec --json -m gpt-5.6-luna" {
		t.Fatalf("baseline command = %#v metadata=%+v err=%v", args, metadata, err)
	}

	args, metadata, err = workerCommand("mimo-v2.5")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{"-m mimo-v2.5", "model_provider=\"mimo\"", "model_providers.mimo.wire_api=\"responses\"", "model_providers.mimo.base_url=\"https://token-plan-ams.xiaomimimo.com/v1\"", "web_search=\"disabled\""} {
		if !strings.Contains(joined, want) {
			t.Fatalf("MiMo command missing %q: %s", want, joined)
		}
	}
	if metadata.Provider != "Xiaomi MiMo" || metadata.BillingClass != "Token Plan" || metadata.IdentityConfidence != "LOW" {
		t.Fatalf("unexpected MiMo metadata: %+v", metadata)
	}
	if metadata.Protocol != "responses" || !metadata.ExplicitOnly {
		t.Fatalf("unexpected MiMo route metadata: %+v", metadata)
	}

	for _, profile := range []string{"gpt-6-astra", "gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.5"} {
		args, metadata, err := workerCommand(profile)
		if err != nil {
			t.Fatalf("%s: %v", profile, err)
		}
		if got := strings.Join(args, " "); got != "exec --json -m "+profile {
			t.Fatalf("%s command = %q", profile, got)
		}
		if metadata.Provider != "OpenAI" || metadata.Harness != "Codex CLI" || metadata.Protocol != "codex-native" || metadata.Certification != "candidate" || metadata.IdentityConfidence != "UNVERIFIED" || !metadata.ExplicitOnly {
			t.Fatalf("unexpected %s metadata: %+v", profile, metadata)
		}
	}

	for _, profile := range []string{"opencode-go/muse-spark-1.3-contributor", "deepseek/deepseek-v4-flash", "cheaper-inference/deepseek-v4-flash"} {
		args, metadata, err := workerCommand(profile)
		if err != nil {
			t.Fatalf("%s: %v", profile, err)
		}
		if got := strings.Join(args, " "); got != "run --format json --model "+profile {
			t.Fatalf("%s command = %q", profile, got)
		}
		if metadata.Harness != "OpenCode CLI" || metadata.Executable != "opencode" || metadata.LaunchMode != "opencode" || metadata.CredentialStore != "OpenCode local auth" || !metadata.ExplicitOnly {
			t.Fatalf("unexpected %s metadata: %+v", profile, metadata)
		}
	}

	for _, profile := range []string{"antigravity/gemini-3.8-flash-high", "antigravity/gemini-3.8-flash-medium", "antigravity/gemini-3.8-flash-low"} {
		args, metadata, err := workerCommand(profile)
		if err != nil {
			t.Fatalf("%s: %v", profile, err)
		}
		model := strings.TrimPrefix(profile, "antigravity/")
		if got := strings.Join(args, " "); got != "--output-format json --model "+model {
			t.Fatalf("%s command = %q", profile, got)
		}
		if metadata.Harness != "AGY CLI" || metadata.Executable != "agy" || metadata.LaunchMode != "antigravity" || metadata.CredentialStore != "Antigravity local session" || !metadata.ExplicitOnly {
			t.Fatalf("unexpected %s metadata: %+v", profile, metadata)
		}
	}
}

func TestCandidateProfilesRequireExplicitSelection(t *testing.T) {
	t.Setenv("MIMO_API_KEY", "test-only-not-a-real-key")
	for _, profile := range []string{"gpt-6-astra", "gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.5", "opencode-go/muse-spark-1.3-contributor", "deepseek/deepseek-v4-flash", "cheaper-inference/deepseek-v4-flash", "antigravity/gemini-3.8-flash-high", "antigravity/gemini-3.8-flash-medium", "antigravity/gemini-3.8-flash-low", "mimo-v2.5"} {
		_, metadata, err := workerCommand(profile)
		if err != nil {
			t.Fatalf("%s: %v", profile, err)
		}
		if err := validateProfileSelection(JobSpec{ID: "candidate-test"}, profile, metadata); err == nil || !strings.Contains(err.Error(), "candidate-only") {
			t.Fatalf("%s was silently selectable: %v", profile, err)
		}
		if err := validateProfileSelection(JobSpec{ID: "candidate-test", WorkerProfile: profile}, profile, metadata); err != nil {
			t.Fatalf("explicit %s selection rejected: %v", profile, err)
		}
	}
}

func TestWorkerCommandRejectsUnconfiguredMiMo(t *testing.T) {
	t.Setenv("MIMO_API_KEY", "")
	if _, _, err := workerCommand("mimo-v2.5"); err == nil || !strings.Contains(err.Error(), "MIMO_API_KEY") {
		t.Fatalf("expected concrete missing-credential error, got %v", err)
	}
}

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
	spec := JobSpec{ID: "acceptance-test", RepoPath: root, BaseRef: "main", Task: "test acceptance", Acceptance: []string{"git diff --check"}}
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

func TestEventJournalIsDurableAndIdempotent(t *testing.T) {
	root := t.TempDir()
	m, err := NewManager(JobSpec{ID: "events", RepoPath: root, BaseRef: "main", Task: "event test", Acceptance: []string{"exit 0"}, Preflight: "GARY PREFLIGHT"}, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	if err := m.SetStatus("DONE"); err != nil {
		t.Fatal(err)
	}
	if err := m.SetStatus("DONE"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(m.State().EventsPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 4 {
		t.Fatalf("journal events=%d, want JOB_CREATED/PREFLIGHT/STATUS_CHANGED/JOB_DONE exactly once", len(lines))
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(m.State().StatePath), "events.JOB_NOTE.md")); err != nil {
		t.Fatalf("derived job note was not refreshed: %v", err)
	}
	seen := map[string]bool{}
	for _, line := range lines {
		var event Event
		if json.Unmarshal([]byte(line), &event) != nil {
			t.Fatal("invalid event")
		}
		if seen[event.EventID] {
			t.Fatalf("duplicate event %q", event.EventID)
		}
		seen[event.EventID] = true
	}
}

func TestSecondWorkerRequiresIndependenceAndThirdIsRejected(t *testing.T) {
	root := t.TempDir()
	m, err := NewManager(JobSpec{ID: "workers", RepoPath: root, BaseRef: "main", Task: "parallel test", Acceptance: []string{"exit 0"}}, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	m.state.Workers = []*WorkerRecord{
		{ID: "one", JobID: "workers", Profile: "gpt-5.6-luna", Task: "one", Status: "RUNNING", PID: os.Getpid(), Worktree: root},
		{ID: "two", JobID: "workers", Profile: "gpt-5.6-luna", Task: "two", Status: "RUNNING", PID: os.Getpid(), Worktree: root},
	}
	m.state.Worker = m.state.Workers[1]
	if err := m.saveLocked(); err != nil {
		t.Fatal(err)
	}
	if _, err := m.SpawnWorkerBounded("three", "gpt-5.6-luna", true); err == nil || !strings.Contains(err.Error(), "maximum of two") {
		t.Fatalf("third worker was not rejected: %v", err)
	}
}

func TestSmallFixBoundaryRejectsOversizedPatch(t *testing.T) {
	patch := "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n"
	for i := 0; i < 41; i++ {
		patch += "+line\n"
	}
	if _, changed := patchStats(patch); changed != 41 {
		t.Fatalf("changed lines=%d", changed)
	}
	if files, changed := patchStats(patch); files != 1 || changed <= 40 {
		t.Fatalf("bad patch stats files=%d changed=%d", files, changed)
	}
}
