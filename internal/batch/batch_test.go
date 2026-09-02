package batch

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/doctrine"
)

func TestGenerateJobConfig(t *testing.T) {
	cfg := JobConfig{
		Project:      "test-project",
		Region:       "europe-west2",
		JobName:      "gary-smoke-20260101-120000",
		Heartbeat:    "GARY_HEARTBEAT_12345",
		MachineType:  "e2-small",
		MaxRunDurSec: 300,
		MaxRetries:   0,
	}

	data, err := GenerateJobConfig(cfg)
	if err != nil {
		t.Fatalf("GenerateJobConfig: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	taskGroups, ok := parsed["taskGroups"].([]interface{})
	if !ok || len(taskGroups) != 1 {
		t.Fatalf("expected 1 taskGroup, got %v", parsed["taskGroups"])
	}

	tg := taskGroups[0].(map[string]interface{})
	if tg["taskCount"] != "1" {
		t.Errorf("taskCount = %v, want 1", tg["taskCount"])
	}
	if tg["parallelism"] != "1" {
		t.Errorf("parallelism = %v, want 1", tg["parallelism"])
	}

	ts := tg["taskSpec"].(map[string]interface{})
	if ts["maxRetryCount"] != float64(0) {
		t.Errorf("maxRetryCount = %v, want 0", ts["maxRetryCount"])
	}
	if ts["maxRunDuration"] != "300s" {
		t.Errorf("maxRunDuration = %v, want 300s", ts["maxRunDuration"])
	}

	runnables := ts["runnables"].([]interface{})
	script := runnables[0].(map[string]interface{})["script"].(map[string]interface{})
	text := script["text"].(string)
	if !strings.Contains(text, "GARY_HEARTBEAT_12345") {
		t.Errorf("script text missing heartbeat: %s", text)
	}

	alloc := parsed["allocationPolicy"].(map[string]interface{})
	instances := alloc["instances"].([]interface{})
	policy := instances[0].(map[string]interface{})["policy"].(map[string]interface{})
	if policy["machineType"] != "e2-small" {
		t.Errorf("machineType = %v, want e2-small", policy["machineType"])
	}
	if policy["provisioningModel"] != "STANDARD" {
		t.Errorf("provisioningModel = %v, want STANDARD", policy["provisioningModel"])
	}

	logsPolicy := parsed["logsPolicy"].(map[string]interface{})
	if logsPolicy["destination"] != "CLOUD_LOGGING" {
		t.Errorf("logsPolicy.destination = %v, want CLOUD_LOGGING", logsPolicy["destination"])
	}
}

func TestGenerateJobConfigDefaults(t *testing.T) {
	cfg := JobConfig{
		Project:   "p",
		Region:    "r",
		JobName:   "j",
		Heartbeat: "h",
	}

	data, err := GenerateJobConfig(cfg)
	if err != nil {
		t.Fatalf("GenerateJobConfig: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	alloc := parsed["allocationPolicy"].(map[string]interface{})
	instances := alloc["instances"].([]interface{})
	policy := instances[0].(map[string]interface{})["policy"].(map[string]interface{})
	if policy["machineType"] != "e2-small" {
		t.Errorf("default machineType = %v, want e2-small", policy["machineType"])
	}

	tg := parsed["taskGroups"].([]interface{})[0].(map[string]interface{})
	ts := tg["taskSpec"].(map[string]interface{})
	if ts["maxRunDuration"] != "300s" {
		t.Errorf("default maxRunDuration = %v, want 300s", ts["maxRunDuration"])
	}
	if ts["maxRetryCount"] != float64(0) {
		t.Errorf("default maxRetryCount = %v, want 0", ts["maxRetryCount"])
	}
}

func TestIsTerminal(t *testing.T) {
	tests := []struct {
		state string
		want  bool
	}{
		{"SUCCEEDED", true},
		{"FAILED", true},
		{"CANCELLED", true},
		{"DELETING", true},
		{"DELETED", true},
		{"QUEUED", false},
		{"SCHEDULED", false},
		{"RUNNING", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			if got := IsTerminal(tt.state); got != tt.want {
				t.Errorf("IsTerminal(%q) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

func TestSmokeResultJSON(t *testing.T) {
	result := SmokeResult{
		Kind:              "gary-smoke",
		JobID:             "gary-smoke-20260101-120000",
		JobUID:            "abc-123",
		Project:           "test-project",
		Region:            "europe-west2",
		MachineType:       "e2-small",
		ProvisioningModel: "STANDARD",
		SubmittedAt:       "2026-01-01T12:00:00Z",
		FinishedAt:        "2026-01-01T12:01:00Z",
		DurationSeconds:   60,
		FinalState:        "SUCCEEDED",
		Heartbeat:         "GARY_HEARTBEAT_12345",
		HeartbeatVerified: true,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}

	var parsed SmokeResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if parsed.Kind != "gary-smoke" {
		t.Errorf("Kind = %q, want gary-smoke", parsed.Kind)
	}
	if parsed.HeartbeatVerified != true {
		t.Errorf("HeartbeatVerified = %v, want true", parsed.HeartbeatVerified)
	}
	if parsed.DurationSeconds != 60 {
		t.Errorf("DurationSeconds = %v, want 60", parsed.DurationSeconds)
	}
}

func TestParseJobStatusRealistic(t *testing.T) {
	raw := `{
  "name": "projects/resolve-ai-agentic-2608151746/locations/europe-west2/jobs/gary-smoke-20260824-120000",
  "uid": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "priority": 0,
  "taskGroups": [],
  "allocationPolicy": {},
  "labels": {"gary-build": "2b"},
  "status": {
    "state": "SUCCEEDED"
  },
  "createTime": "2026-08-24T12:00:00Z",
  "updateTime": "2026-08-24T12:01:00Z"
}`

	var status JobStatus
	if err := json.Unmarshal([]byte(raw), &status); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if status.UID != "a1b2c3d4-e5f6-7890-abcd-ef1234567890" {
		t.Errorf("UID = %q, want a1b2c3d4-e5f6-7890-abcd-ef1234567890", status.UID)
	}
	if status.Name != "projects/resolve-ai-agentic-2608151746/locations/europe-west2/jobs/gary-smoke-20260824-120000" {
		t.Errorf("Name = %q, want full job path", status.Name)
	}
	if status.State() != "SUCCEEDED" {
		t.Errorf("State() = %q, want SUCCEEDED", status.State())
	}
	if !IsTerminal(status.State()) {
		t.Errorf("SUCCEEDED should be terminal")
	}
}

func TestParseJobStatusRunning(t *testing.T) {
	raw := `{
  "name": "projects/p/locations/r/jobs/j",
  "uid": "uid-123",
  "status": {
    "state": "RUNNING"
  }
}`

	var status JobStatus
	if err := json.Unmarshal([]byte(raw), &status); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if status.State() != "RUNNING" {
		t.Errorf("State() = %q, want RUNNING", status.State())
	}
	if IsTerminal(status.State()) {
		t.Errorf("RUNNING should not be terminal")
	}
}

func TestHeartbeatInScript(t *testing.T) {
	heartbeats := []string{
		"GARY_HEARTBEAT_12345",
		"GARY_HEARTBEAT_9999999999999",
		"GARY_HEARTBEAT_test-with-dashes",
	}

	for _, hb := range heartbeats {
		t.Run(hb, func(t *testing.T) {
			cfg := JobConfig{
				Project:   "p",
				Region:    "r",
				JobName:   "j",
				Heartbeat: hb,
			}
			data, err := GenerateJobConfig(cfg)
			if err != nil {
				t.Fatalf("GenerateJobConfig: %v", err)
			}
			if !strings.Contains(string(data), hb) {
				t.Errorf("heartbeat %q not found in generated config", hb)
			}
		})
	}
}

func TestCodingWorkerDisablesHiddenGeminiRetries(t *testing.T) {
	cfg := CodingJobConfig{
		Project: "p", Region: "r", JobName: "j",
		RepoURL: "https://example.com/repo.git", BaseBranch: "main",
		GaryBranch: "gary/test", Task: "do stuff", Acceptance: "echo ok",
	}
	script := generateCodingWorkerScript(cfg)
	for _, required := range []string{
		"GEMINI_CLI_SYSTEM_SETTINGS_PATH",
		"\"retryFetchErrors\":false",
		"\"maxAttempts\":1",
		"\"modelRouting\":false",
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("worker script missing %q", required)
		}
	}
}

func TestCodingWorkerIsSteerableAndResumable(t *testing.T) {
	script := generateCodingWorkerScript(CodingJobConfig{
		RepoURL: "https://example.com/repo.git", BaseBranch: "main", GaryBranch: "gary/test",
		Task: "implement the coherent outcome", Acceptance: "echo ok", GCSPath: "gs://bucket/run",
		Model: "gemini-3.5-flash-lite",
	})
	for _, required := range []string{
		"emit_telemetry running",
		"sleep 60",
		"apply_running_control",
		"STOP_AND_REPLAN",
		"READY_FOR_INSPECTION",
		"--resume \"$SESSION_ID\"",
		"Waiting for Yardmaster review after turn",
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("supervised worker script missing %q", required)
		}
	}
	if strings.Contains(script, "480s gemini") || strings.Contains(script, "SOFT DEADLINE") {
		t.Fatal("ordinary worker execution must not be killed by the old fixed deadline")
	}
}

func TestCodingJobPassesOnlyShortLivedTokenToWorker(t *testing.T) {
	cfg := CodingJobConfig{
		Project: "p", Region: "r", JobName: "j",
		RepoURL:    "https://github.com/matthewjameswatkins1978-cyber/book-burner.git",
		BaseBranch: "main", GaryBranch: "gary/test", Task: "do stuff",
		Acceptance: "echo ok", GitHubToken: "installation-token",
	}
	data, err := GenerateCodingJobConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "GARY_GITHUB_TOKEN") || !strings.Contains(text, "installation-token") {
		t.Fatal("worker config did not contain the short-lived repository token")
	}
	for _, forbidden := range []string{"GARY_GITHUB_APP_PRIVATE_KEY", "BEGIN RSA PRIVATE KEY", "BEGIN PRIVATE KEY"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("worker config must not contain App private key material: %q", forbidden)
		}
	}
}

func TestCodingJobAcceptanceUsesExitCode(t *testing.T) {
	cfg := CodingJobConfig{
		Project:    "p",
		Region:     "r",
		JobName:    "j",
		RepoURL:    "https://example.com/repo.git",
		BaseBranch: "main",
		GaryBranch: "gary/test",
		Task:       "do stuff",
		Acceptance: "python -m pytest -q",
	}

	script := generateCodingWorkerScript(cfg)

	// Must use exit code, not grep
	if strings.Contains(script, "grep -q") {
		t.Error("coding job script must not use grep for acceptance detection")
	}
	// Must capture exit code
	if !strings.Contains(script, "ACCEPTANCE_EXIT=$?") {
		t.Error("coding job script must capture acceptance exit code")
	}
	// Must check exit code 0
	if !strings.Contains(script, "$ACCEPTANCE_EXIT -eq 0") {
		t.Error("coding job script must check exit code equals 0")
	}
}

func TestCodingJobPreservesFailedAcceptanceEvidenceBeforeExit(t *testing.T) {
	cfg := CodingJobConfig{
		Project: "p", Region: "r", JobName: "j",
		RepoURL: "https://example.com/repo.git", BaseBranch: "main",
		GaryBranch: "gary/test", Task: "fix it", Acceptance: "python -m pytest -q",
		GCSPath: "gs://bucket/run",
	}
	script := generateCodingWorkerScript(cfg)

	for _, required := range []string{
		": > /workspace/acceptance_output.txt",
		"set +e",
		"ACCEPTANCE_EXIT=$?",
		"set -e",
		"GARY_ACCEPTANCE_B64_START",
		"/workspace/usage.jsonl",
		"--output-format stream-json",
		"GARY_USAGE_B64_START",
		"gcloud storage cp /workspace/acceptance_output.txt",
		"gcloud storage cp /workspace/usage.jsonl",
		"ACCEPTANCE FAILED (exit code $ACCEPTANCE_EXIT)",
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("worker script missing %q", required)
		}
	}

	completionIndex := strings.LastIndex(script, "echo \"GARY CODING WORKER COMPLETE\"")
	markerIndex := strings.Index(script, "GARY_ACCEPTANCE_B64_START")
	uploadIndex := strings.Index(script, "gcloud storage cp /workspace/acceptance_output.txt")
	if completionIndex < markerIndex || completionIndex < uploadIndex {
		t.Fatal("failed acceptance must emit and upload evidence before completing")
	}
	if strings.Contains(script, "exit $ACCEPTANCE_EXIT") {
		t.Fatal("acceptance failure must remain evidence, not Batch infrastructure failure")
	}
	if !strings.Contains(script, "exit 0") {
		t.Fatal("worker must complete successfully after capturing acceptance evidence")
	}
}

func TestCodingWorkerRustCanaryContract(t *testing.T) {
	cfg := CodingJobConfig{
		Project: "p", Region: "r", JobName: "j",
		RepoURL: "https://example.com/repo.git", BaseBranch: "main",
		GaryBranch: "gary/canary", Task: "create a trivial Rust source file",
		Acceptance: "test -f src/canary.rs",
		GCSPath:    "gs://bucket/run",
	}
	script := generateCodingWorkerScript(cfg)
	for _, required := range []string{
		"Rust: $(rustc --version)",
		"Cargo: $(cargo --version)",
		"git format-patch",
		"GARY_PATCH_B64_START",
		"GARY_PATCH_B64_END",
		"GARY CODING WORKER COMPLETE",
		"exit 0",
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("Rust canary worker contract missing %q", required)
		}
	}
}

func TestCodingJobCommitMessageIsShort(t *testing.T) {
	cfg := CodingJobConfig{
		Project:    "p",
		Region:     "r",
		JobName:    "j",
		RepoURL:    "https://example.com/repo.git",
		BaseBranch: "main",
		GaryBranch: "gary/test",
		Task:       "fix the bug",
		Acceptance: "echo ok",
	}

	script := generateCodingWorkerScript(cfg)

	// Must use short stable commit message
	if !strings.Contains(script, "GARY worker result") {
		t.Error("worker commit message should be 'GARY worker result'")
	}

	// The git commit command line must use the literal message, not task text.
	var commitLine string
	for _, line := range strings.Split(script, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "git commit") {
			commitLine = trimmed
			break
		}
	}
	if commitLine == "" {
		t.Fatal("no git commit line found in script")
	}
	if !strings.Contains(commitLine, "GARY worker result") {
		t.Errorf("git commit line should contain 'GARY worker result', got: %s", commitLine)
	}
	if strings.Contains(commitLine, cfg.Task) {
		t.Error("git commit line must not contain task text")
	}
}

func TestObservedRuntimeSecondsUsesProviderTimesThenFallback(t *testing.T) {
	status := &JobStatus{CreateTime: "2026-08-30T10:00:00Z", UpdateTime: "2026-08-30T10:02:30Z"}
	if got := ObservedRuntimeSeconds(status, time.Time{}, time.Time{}); got != 150 {
		t.Fatalf("provider runtime = %.0f, want 150", got)
	}

	start := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	finish := start.Add(42 * time.Second)
	status = &JobStatus{CreateTime: "not-a-time", UpdateTime: "not-a-time"}
	if got := ObservedRuntimeSeconds(status, start, finish); got != 42 {
		t.Fatalf("fallback runtime = %.0f, want 42", got)
	}
}

func TestCodingJobAcceptancePytestStyle(t *testing.T) {
	// Pytest outputs lowercase "passed" — must NOT affect acceptance.
	// Only exit code matters.
	cfg := CodingJobConfig{
		Project:    "p",
		Region:     "r",
		JobName:    "j",
		RepoURL:    "https://example.com/repo.git",
		BaseBranch: "main",
		GaryBranch: "gary/test",
		Task:       "fix tests",
		Acceptance: "python -m pytest -q",
	}

	script := generateCodingWorkerScript(cfg)

	// The script must not parse "passed" or "PASS" from output
	if strings.Contains(script, "grep") && strings.Contains(script, "PASS") {
		t.Error("coding job must not grep for PASS/PASS in acceptance output")
	}
}

func TestCodingWorkerPromptIncludesCorrectRoleUniversalRulesAndTask(t *testing.T) {
	task := "Fix one bounded defect."
	acceptance := "go test ./..."
	prompt := codingWorkerPrompt(task, acceptance, 5)
	if !strings.Contains(prompt, doctrine.CompletionPrinciple) {
		t.Fatal("worker prompt must include the completion principle")
	}
	if !strings.Contains(prompt, doctrine.WorkerRoleRules) {
		t.Fatal("worker prompt must include Worker role rules")
	}
	if strings.Contains(prompt, doctrine.YardmasterRoleRules) {
		t.Fatal("worker prompt must not include Yardmaster role rules")
	}
	if !strings.Contains(prompt, doctrine.UniversalRules) {
		t.Fatal("worker prompt must include universal GARY rules")
	}
	if !strings.Contains(prompt, "Keep generated environments, caches, build products") {
		t.Fatal("worker prompt must include repository hygiene rules")
	}
	if !strings.Contains(prompt, "You are live-supervised by the Yardmaster") || !strings.Contains(prompt, "Elapsed time is not failure") {
		t.Fatal("worker prompt must describe live supervision instead of a runtime deadline")
	}
	if !strings.Contains(prompt, "YARDMASTER ESTIMATE: this coherent outcome is expected to take about 5 minutes") {
		t.Fatal("worker prompt must include Yardmaster's advisory estimate")
	}
	if !strings.Contains(prompt, task) {
		t.Fatal("worker prompt must preserve the bounded job")
	}
	if !strings.Contains(prompt, acceptance) {
		t.Fatal("worker prompt must preserve the Yardmaster checks")
	}
	if strings.Contains(prompt, "Do not create virtual environments, install dependencies") {
		t.Fatal("worker prompt must leave ordinary tool setup to Worker judgment")
	}
}

func TestCodingWorkerPromptOnlyIncludesValidYardmasterEstimate(t *testing.T) {
	for _, test := range []struct {
		name     string
		want     string
		estimate int
	}{
		{name: "valid", want: "YARDMASTER ESTIMATE: this coherent outcome is expected to take about 6 minutes", estimate: 6},
		{name: "long-valid", want: "YARDMASTER ESTIMATE: this coherent outcome is expected to take about 30 minutes", estimate: 30},
		{name: "missing", estimate: 0},
		{name: "out-of-range", estimate: 61},
	} {
		t.Run(test.name, func(t *testing.T) {
			prompt := codingWorkerPrompt("bounded task", "go test ./...", test.estimate)
			if test.want != "" && !strings.Contains(prompt, test.want) {
				t.Fatalf("prompt missing estimate marker %q", test.want)
			}
			if test.want == "" && strings.Contains(prompt, "YARDMASTER ESTIMATE") {
				t.Fatalf("prompt included invalid estimate %d", test.estimate)
			}
		})
	}
}

func TestCodingWorkerScriptCleansDisposableArtifacts(t *testing.T) {
	script := generateCodingWorkerScript(CodingJobConfig{RepoURL: "https://example.com/repo.git", BaseBranch: "main", GaryBranch: "gary/job", Task: "task", Acceptance: "echo ok"})
	for _, marker := range []string{"-name .venv", "-name __pycache__", "-name .build", "-name target", "telemetry.json", "control.json", "--resume", "Yardmaster supervision: CONTINUE", "git add -A"} {
		if !strings.Contains(script, marker) {
			t.Fatalf("worker script missing artifact hygiene marker %q", marker)
		}
	}
}

func TestPromptTransportRoundTrip(t *testing.T) {
	// B3: Task text must survive base64 encode → shell decode byte-for-byte.
	specialTask := `Fix "process_items" so it handles:
- double quote: "
- single quote: '
- dollar sign: $PATH
- backtick: ` + "`" + `cmd` + "`" + `
- JSON: {"key": "value", "nested": {"a": 1}}
- multiple lines
  with indentation
  and special chars: <>&|;`

	cfg := CodingJobConfig{
		Project:    "p",
		Region:     "r",
		JobName:    "j",
		RepoURL:    "https://example.com/repo.git",
		BaseBranch: "main",
		GaryBranch: "gary/test",
		Task:       specialTask,
		Acceptance: "echo ok",
	}

	// Build the full prompt the same way GenerateCodingJobConfig does.
	prompt := codingWorkerPrompt(cfg.Task, cfg.Acceptance, cfg.EstimatedMinutes)

	promptB64 := base64.StdEncoding.EncodeToString([]byte(prompt))

	// Simulate shell decode
	decoded, err := base64.StdEncoding.DecodeString(promptB64)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	if string(decoded) != prompt {
		t.Errorf("round-trip mismatch\ngot:\n%s\nwant:\n%s", string(decoded), prompt)
	}
}

func TestPromptShellSafe(t *testing.T) {
	// B3: The base64-encoded prompt must not appear as raw shell syntax.
	tasks := []string{
		`has "double quotes"`,
		`has 'single quotes'`,
		"has $DOLLAR",
		"has `backticks`",
		`has {"json": true}`,
		"has\nmultiple\nlines",
		"has <redirection> and |pipe| and ;semi",
	}

	for _, task := range tasks {
		name := task
		if len(name) > 20 {
			name = name[:20]
		}
		t.Run(name, func(t *testing.T) {
			cfg := CodingJobConfig{
				Project:    "p",
				Region:     "r",
				JobName:    "j",
				RepoURL:    "https://example.com/repo.git",
				BaseBranch: "main",
				GaryBranch: "gary/test",
				Task:       task,
				Acceptance: "echo ok",
			}

			// GenerateCodingJobConfig must succeed with no error
			data, err := GenerateCodingJobConfig(cfg)
			if err != nil {
				t.Fatalf("GenerateCodingJobConfig failed: %v", err)
			}

			// The raw task text must NOT appear in the JSON config
			// (it should be base64 encoded)
			if strings.Contains(string(data), task) {
				t.Errorf("raw task text must not appear in Batch config JSON")
			}
		})
	}
}
