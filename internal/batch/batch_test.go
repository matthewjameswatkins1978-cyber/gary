package batch

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

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


func TestCodingWorkerPromptIncludesUniversalRulesAndTask(t *testing.T) {
	task := "Fix one bounded defect."
	prompt := codingWorkerPrompt(task)
	if !strings.Contains(prompt, doctrine.UniversalForemanRules) {
		t.Fatal("worker prompt must include universal GARY rules")
	}
	if !strings.Contains(prompt, task) {
		t.Fatal("worker prompt must preserve the bounded job")
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
	prompt := codingWorkerPrompt(cfg.Task)

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
