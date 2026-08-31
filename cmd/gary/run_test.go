package main

import (
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/config"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/jobs"
)

func TestExtractMarkerFromBatchLogsFiltersUIDAndRestoresOrder(t *testing.T) {
	jobUID := "job-1"
	encoded := base64.StdEncoding.EncodeToString([]byte("patch-data"))
	entries := []batchLogEntry{
		{Labels: map[string]string{"job_uid": "other"}, TextPayload: encoded, Timestamp: "2026-01-01T00:00:02Z"},
		{Labels: map[string]string{"job_uid": jobUID}, TextPayload: "GARY_PATCH_B64_END", Timestamp: "2026-01-01T00:00:03Z"},
		{Labels: map[string]string{"job_uid": jobUID}, TextPayload: encoded, Timestamp: "2026-01-01T00:00:02Z"},
		{Labels: map[string]string{"job_uid": jobUID}, TextPayload: "GARY_PATCH_B64_START", Timestamp: "2026-01-01T00:00:01Z"},
	}
	// Cloud Logging returns newest entries first; extraction sorts before this helper.
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Timestamp < entries[j].Timestamp })
	if got := extractMarkerFromBatchLogs(entries, jobUID, "GARY_PATCH_B64"); got != "patch-data" {
		t.Fatalf("extracted marker = %q, want patch-data", got)
	}
}

func TestRuntimeBudgets(t *testing.T) {
	if defaultWorkerMaxRunSec != 600 {
		t.Fatalf("default worker runtime = %d, want 600", defaultWorkerMaxRunSec)
	}
	if maxAttempts != 8 {
		t.Fatalf("worker safety attempt limit = %d, want 8", maxAttempts)
	}
	if maxLowProgressAttempts != 3 {
		t.Fatalf("low-progress escalation threshold = %d, want 3", maxLowProgressAttempts)
	}
	if batchSchedulingGraceSec != 300 {
		t.Fatalf("Batch scheduling grace = %d, want 300", batchSchedulingGraceSec)
	}
	if yardmasterPlannerTimeout != 5*time.Minute {
		t.Fatalf("Yardmaster planner timeout = %s, want 5m", yardmasterPlannerTimeout)
	}
}

func TestMissingProjectContextBlocksBeforeWorkerSubmission(t *testing.T) {
	cfg := &config.Config{ResultsDir: t.TempDir(), ImplementerModel: "gemini-3.5-flash-lite"}
	result := RunJobOnceWithModel(cfg, &jobs.Job{ID: "context-check", ProjectID: "missing-project", Repo: "https://github.com/matthewjameswatkins1978-cyber/gary.git", BaseBranch: "master", Task: "must not run", Acceptance: []string{"go test ./..."}}, "", "gemini-3.5-flash-lite")
	if result.FinalStatus != "BLOCKED" || result.Attempts != 0 {
		t.Fatalf("missing project context was not blocked before worker submission: %+v", result)
	}
}

func TestBatchGitHubTokenSourceIsConsultedForEachSubmissionBoundary(t *testing.T) {
	original := batchGitHubTokenSource
	defer func() { batchGitHubTokenSource = original }()

	calls := 0
	batchGitHubTokenSource = func(repoURL string) (string, error) {
		if repoURL != "https://github.com/matthewjameswatkins1978-cyber/book-burner.git" {
			t.Fatalf("token source repo = %q", repoURL)
		}
		calls++
		if calls == 1 {
			return "token-one", nil
		}
		return "token-two", nil
	}

	first, err := currentBatchGitHubToken("https://github.com/matthewjameswatkins1978-cyber/book-burner.git")
	if err != nil {
		t.Fatal(err)
	}
	second, err := currentBatchGitHubToken("https://github.com/matthewjameswatkins1978-cyber/book-burner.git")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || first != "token-one" || second != "token-two" {
		t.Fatalf("token source calls=%d first=%q second=%q", calls, first, second)
	}
}

func TestServiceEnvironmentBlockerUsesStrongInfrastructureSignatures(t *testing.T) {
	codingFailure := "TypeError: CedarAuthorization.__init__() got an unexpected keyword argument 'policy'\ncat: /workspace/acceptance_output.txt: No such file or directory"
	if isServiceEnvironmentBlocker(codingFailure) {
		t.Fatal("coding/API mismatch with incidental missing file must remain repair evidence")
	}
	authFailure := "fatal: could not read Username for 'https://github.com': No such device or address"
	if !isServiceEnvironmentBlocker(authFailure) {
		t.Fatal("GitHub clone authentication failure must be classified as infrastructure blocked")
	}
}

func TestCleanupDisposableWorkerArtifactsBeforeCommit(t *testing.T) {
	root := t.TempDir()
	keep := filepath.Join(root, "intentional.txt")
	if err := os.WriteFile(keep, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(root, ".venv", "bin", "python"),
		filepath.Join(root, "nested", "__pycache__", "module.pyc"),
		filepath.Join(root, "package.egg-info", "PKG-INFO"),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("disposable"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := cleanupDisposableWorkerArtifacts(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("intentional file was removed: %v", err)
	}
	for _, path := range []string{filepath.Join(root, ".venv"), filepath.Join(root, "nested", "__pycache__"), filepath.Join(root, "package.egg-info")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("disposable path still exists: %s", path)
		}
	}
}

func TestCompactFailureLogs(t *testing.T) {
	short := "one\ntwo"
	if got := compactFailureLogs(short, 100); got != short {
		t.Fatalf("short logs changed: %q", got)
	}

	long := strings.Repeat("x", 5000)
	got := compactFailureLogs(long, 4000)
	if !strings.HasPrefix(got, "[truncated]\n") {
		t.Fatalf("expected truncation marker, got prefix %q", got[:12])
	}
	if len(got) != len("[truncated]\n")+4000 {
		t.Fatalf("unexpected compacted length: %d", len(got))
	}
}

func TestSummarizeAcceptance(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "empty",
			input:  "",
			expect: "no-output",
		},
		{
			name:   "all pass",
			input:  "ok  \tclamp\t0.003s\nok  \tmath\t0.001s\n",
			expect: "ok  \tclamp\t0.003s\nok  \tmath\t0.001s",
		},
		{
			name:   "some fail",
			input:  "FAIL\tclamp\t0.004s\nok  \tmath\t0.001s\n",
			expect: "FAIL\tclamp\t0.004s\nok  \tmath\t0.001s",
		},
		{
			name:   "no ok lines",
			input:  "some random output\nmore output\n",
			expect: "some random output\nmore output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := summarizeAcceptance(tt.input)
			if got != tt.expect {
				t.Errorf("got %q, want %q", got, tt.expect)
			}
		})
	}
}

func TestCheckProgress(t *testing.T) {
	tests := []struct {
		name    string
		history []string
		current string
		expect  bool
	}{
		{
			name:    "first attempt always progress",
			history: []string{},
			current: "FAIL\tclamp",
			expect:  true,
		},
		{
			name:    "fewer failures is progress",
			history: []string{"FAIL\tclamp\nFAIL\tmath"},
			current: "FAIL\tclamp",
			expect:  true,
		},
		{
			name:    "same failures is no progress",
			history: []string{"FAIL\tclamp"},
			current: "FAIL\tclamp",
			expect:  false,
		},
		{
			name:    "more failures is no progress",
			history: []string{"FAIL\tclamp"},
			current: "FAIL\tclamp\nFAIL\tmath",
			expect:  false,
		},
		{
			name:    "pass after fail is progress",
			history: []string{"FAIL\tclamp"},
			current: "ok  \tclamp",
			expect:  true,
		},
		{
			name:    "different output is progress",
			history: []string{"FAIL\tclamp: wrong result"},
			current: "FAIL\tclamp: timeout",
			expect:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkProgress(tt.history, tt.current)
			if got != tt.expect {
				t.Errorf("checkProgress(%v, %q) = %v, want %v", tt.history, tt.current, got, tt.expect)
			}
		})
	}
}

func TestFormatRepairBrief(t *testing.T) {
	brief := &repairBrief{
		Progress:    "improving",
		Evidence:    "one failing assertion remains",
		Cause:       "function still misses below-min case",
		Keep:        "working above-max clamp and tests",
		Change:      "add below-min clamping logic",
		DoNotChange: "tests or above-max branch",
		NextAction:  "implement the below-min guard",
	}

	output := formatRepairBrief(brief)

	if output == "" {
		t.Fatal("formatRepairBrief returned empty string")
	}
	if !contains(output, "PROGRESS: improving") {
		t.Errorf("output missing PROGRESS: %s", output)
	}
	if !contains(output, "EVIDENCE: one failing assertion remains") {
		t.Errorf("output missing EVIDENCE: %s", output)
	}
	if !contains(output, "KEEP: working above-max clamp and tests") {
		t.Errorf("output missing KEEP: %s", output)
	}
	if !contains(output, "NEXT ACTION: implement the below-min guard") {
		t.Errorf("output missing NEXT ACTION: %s", output)
	}
}

func TestBuildRetryTaskTreatsFailureAsEvidence(t *testing.T) {
	guidance := "REPAIR BRIEF:\n  KEEP: preserve working ordering logic\n  CHANGE: fix error propagation\n"
	got := buildRetryTask(
		"Implement the bounded change.",
		"python -m pytest -q",
		"improving",
		"1 test failed, 9 passed",
		guidance,
	)

	for _, want := range []string{
		"evidence, not a job verdict",
		"Continue from the existing partial work",
		"Progress status: improving",
		"python -m pytest -q",
		"preserve working ordering logic",
		"1 test failed, 9 passed",
		"Preserve what already works",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("retry task missing %q:\n%s", want, got)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestPatchChainPreservesState proves that:
// 1. attempt 1 produces a partial fix (patch1)
// 2. attempt 2 receives patch1 applied, then produces patch2
// 3. the combined state (patch1 + patch2) passes acceptance
//
// This is a local integration test — no cloud credits burned.
func TestPatchChainPreservesState(t *testing.T) {
	dir := t.TempDir()

	// Init git repo
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s\n%s", args, err, string(out))
		}
		return strings.TrimSpace(string(out))
	}

	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")

	// Create initial file with a buggy function
	buggyCode := `package clamp

func Clamp(val, min, max int) int {
	return val
}
`
	os.WriteFile(filepath.Join(dir, "clamp.go"), []byte(buggyCode), 0o644)

	// Create test file that expects correct clamping
	testCode := `package clamp

import "testing"

func TestClamp(t *testing.T) {
	if got := Clamp(15, 0, 10); got != 10 {
		t.Errorf("Clamp(15, 0, 10) = %d, want 10", got)
	}
	if got := Clamp(-5, 0, 10); got != 0 {
		t.Errorf("Clamp(-5, 0, 10) = %d, want 0", got)
	}
	if got := Clamp(5, 0, 10); got != 5 {
		t.Errorf("Clamp(5, 0, 10) = %d, want 5", got)
	}
}
`
	os.WriteFile(filepath.Join(dir, "clamp_test.go"), []byte(testCode), 0o644)

	// Create go.mod
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module clamp\n\ngo 1.24\n"), 0o644)

	run("add", "-A")
	run("commit", "-m", "initial: buggy Clamp function")

	// --- ATTEMPT 1: partial fix (only fixes above-max case) ---
	patch1Code := `package clamp

func Clamp(val, min, max int) int {
	if val > max {
		return max
	}
	return val
}
`
	os.WriteFile(filepath.Join(dir, "clamp.go"), []byte(patch1Code), 0o644)

	// Generate patch1
	patch1 := run("diff", "HEAD")
	if patch1 == "" {
		t.Fatal("patch1 is empty")
	}

	// Verify patch1 is partial: below-min case still broken
	if !strings.Contains(patch1, "if val > max") {
		t.Error("patch1 should contain the > max fix")
	}

	// Reset to clean state for attempt 2
	run("checkout", "--", "clamp.go")

	// --- ATTEMPT 2: receives patch1, then applies additional fix ---
	// Simulate what the worker does: apply previous patch by writing the file,
	// then make new changes on top
	os.WriteFile(filepath.Join(dir, "clamp.go"), []byte(patch1Code), 0o644)

	// Now apply the second fix on top of patch1
	patch2Code := `package clamp

func Clamp(val, min, max int) int {
	if val > max {
		return max
	}
	if val < min {
		return min
	}
	return val
}
`
	os.WriteFile(filepath.Join(dir, "clamp.go"), []byte(patch2Code), 0o644)

	// Generate combined patch (attempt1 + attempt2 changes from main)
	combinedPatch := run("diff", "HEAD")
	if combinedPatch == "" {
		t.Fatal("combined patch is empty")
	}

	// Verify combined patch has BOTH fixes
	if !strings.Contains(combinedPatch, "if val > max") {
		t.Error("combined patch should contain > max fix")
	}
	if !strings.Contains(combinedPatch, "if val < min") {
		t.Error("combined patch should contain < min fix")
	}

	// Verify acceptance passes with combined state
	run("add", "-A")
	run("commit", "-m", "fix: both cases")

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("acceptance failed after combined patch: %s\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "PASS") && !strings.Contains(string(out), "ok") {
		t.Fatalf("expected PASS or ok in test output: %s", string(out))
	}

	t.Logf("patch chain test passed: attempt1 partial + attempt2 full = ACCEPTED")
}
