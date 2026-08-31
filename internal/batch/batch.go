// Package batch isolates Google Cloud Batch submission and status queries.
// It uses os/exec to call documented gcloud CLI commands.
package batch

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/doctrine"
)

// JobConfig holds the parameters needed to submit a Batch smoke job.
type JobConfig struct {
	Project      string
	Region       string
	JobName      string
	Heartbeat    string
	MachineType  string
	MaxRunDurSec int
	MaxRetries   int
}

// JobStatus holds the result of a gcloud batch jobs describe call.
// Google Batch returns state nested under status.state.
type JobStatus struct {
	UID        string        `json:"uid"`
	Name       string        `json:"name"`
	CreateTime string        `json:"createTime"`
	UpdateTime string        `json:"updateTime"`
	Status     JobStatusInfo `json:"status"`
}

// ObservedRuntimeSeconds returns the Batch job lifetime from provider
// timestamps, falling back to the caller's measured interval when older
// responses omit usable timestamps. The result is used only for the explicit
// estimated compute component; GARY does not claim it is invoice data.
func ObservedRuntimeSeconds(status *JobStatus, fallbackStart, fallbackFinish time.Time) float64 {
	if status != nil {
		start, startErr := time.Parse(time.RFC3339Nano, status.CreateTime)
		finish, finishErr := time.Parse(time.RFC3339Nano, status.UpdateTime)
		if startErr == nil && finishErr == nil && finish.After(start) {
			return finish.Sub(start).Seconds()
		}
	}
	if fallbackFinish.After(fallbackStart) {
		return fallbackFinish.Sub(fallbackStart).Seconds()
	}
	return 0
}

// JobStatusInfo holds the nested status from a Batch describe response.
type JobStatusInfo struct {
	State string `json:"state"`
}

// JobSummary is the bounded information exposed by GARY's control plane.
type JobSummary struct {
	Name       string `json:"name"`
	State      string `json:"state"`
	CreateTime string `json:"create_time"`
}

// State returns the top-level job state for convenience.
func (js *JobStatus) State() string {
	return js.Status.State
}

// SmokeResult is the local record written after a successful smoke test.
type SmokeResult struct {
	Kind              string  `json:"kind"`
	JobID             string  `json:"job_id"`
	JobUID            string  `json:"job_uid"`
	Project           string  `json:"project"`
	Region            string  `json:"region"`
	MachineType       string  `json:"machine_type"`
	ProvisioningModel string  `json:"provisioning_model"`
	SubmittedAt       string  `json:"submitted_at"`
	FinishedAt        string  `json:"finished_at"`
	DurationSeconds   float64 `json:"duration_seconds"`
	FinalState        string  `json:"final_state"`
	Heartbeat         string  `json:"heartbeat"`
	HeartbeatVerified bool    `json:"heartbeat_verified"`
}

// TerminalStates are the Batch job states that indicate no further progress.
var TerminalStates = map[string]bool{
	"SUCCEEDED": true,
	"FAILED":    true,
	"CANCELLED": true,
	"DELETING":  true,
	"DELETED":   true,
}

// IsTerminal reports whether the given state is a terminal Batch state.
func IsTerminal(state string) bool {
	return TerminalStates[state]
}

// GenerateJobConfig produces the JSON config content for a Batch job submission.
func GenerateJobConfig(cfg JobConfig) ([]byte, error) {
	maxDur := fmt.Sprintf("%ds", cfg.MaxRunDurSec)
	if cfg.MachineType == "" {
		cfg.MachineType = "e2-small"
	}
	if cfg.MaxRunDurSec == 0 {
		cfg.MaxRunDurSec = 300
		maxDur = "300s"
	}
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 0
	}

	scriptText := "#!/bin/sh\necho \"" + cfg.Heartbeat + "\"\nexit 0\n"

	config := map[string]interface{}{
		"taskGroups": []map[string]interface{}{
			{
				"taskCount":   "1",
				"parallelism": "1",
				"taskSpec": map[string]interface{}{
					"runnables": []map[string]interface{}{
						{
							"script": map[string]interface{}{
								"text": scriptText,
							},
						},
					},
					"maxRunDuration": maxDur,
					"maxRetryCount":  cfg.MaxRetries,
				},
			},
		},
		"allocationPolicy": map[string]interface{}{
			"instances": []map[string]interface{}{
				{
					"policy": map[string]interface{}{
						"machineType":       cfg.MachineType,
						"provisioningModel": "STANDARD",
					},
				},
			},
		},
		"labels": map[string]string{
			"gary-build": "2b",
		},
		"logsPolicy": map[string]interface{}{
			"destination": "CLOUD_LOGGING",
		},
	}

	return json.MarshalIndent(config, "", "  ")
}

// SubmitJob writes the config to a temp file and calls gcloud batch jobs submit.
// Returns the job ID on success.
func SubmitJob(cfg JobConfig) (string, error) {
	data, err := GenerateJobConfig(cfg)
	if err != nil {
		return "", fmt.Errorf("generate config: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "gary-batch-*.json")
	if err != nil {
		return "", fmt.Errorf("create temp config: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("write temp config: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("close temp config: %w", err)
	}

	args := []string{
		"batch", "jobs", "submit", cfg.JobName,
		"--location", cfg.Region,
		"--project", cfg.Project,
		"--config", tmpFile.Name(),
	}

	out, err := exec.Command("gcloud", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gcloud batch submit failed: %s\n%s", err, string(out))
	}

	return cfg.JobName, nil
}

// SubmitCodingJob writes the config to a temp file and calls gcloud batch jobs submit.
func SubmitCodingJob(cfg CodingJobConfig) (string, error) {
	data, err := GenerateCodingJobConfig(cfg)
	if err != nil {
		return "", fmt.Errorf("generate config: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "gary-batch-coding-*.json")
	if err != nil {
		return "", fmt.Errorf("create temp config: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("write temp config: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("close temp config: %w", err)
	}

	args := []string{
		"batch", "jobs", "submit", cfg.JobName,
		"--location", cfg.Region,
		"--project", cfg.Project,
		"--config", tmpFile.Name(),
	}

	out, err := exec.Command("gcloud", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gcloud batch submit failed: %s\n%s", err, string(out))
	}

	return cfg.JobName, nil
}

// DescribeJob calls gcloud batch jobs describe and returns the parsed status.
func DescribeJob(project, region, jobID string) (*JobStatus, error) {
	args := []string{
		"batch", "jobs", "describe", jobID,
		"--location", region,
		"--project", project,
		"--format", "json",
	}

	out, err := exec.Command("gcloud", args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gcloud batch describe failed: %s\n%s", err, string(out))
	}

	var status JobStatus
	if err := json.Unmarshal(out, &status); err != nil {
		return nil, fmt.Errorf("parse job status: %w", err)
	}

	return &status, nil
}

// ListJobs lists recent jobs in one GARY Batch region.
func ListJobs(project, region string, limit int) ([]JobSummary, error) {
	if limit < 1 || limit > 50 {
		return nil, fmt.Errorf("limit must be between 1 and 50")
	}
	out, err := exec.Command("gcloud", "batch", "jobs", "list",
		"--project", project, "--location", region,
		"--limit", fmt.Sprintf("%d", limit),
		"--format", "json(name,status.state,createTime)").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gcloud batch list failed: %w\n%s", err, string(out))
	}
	var raw []struct {
		Name       string `json:"name"`
		CreateTime string `json:"createTime"`
		Status     struct {
			State string `json:"state"`
		} `json:"status"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parse batch list: %w", err)
	}
	items := make([]JobSummary, 0, len(raw))
	for _, job := range raw {
		items = append(items, JobSummary{Name: job.Name, State: job.Status.State, CreateTime: job.CreateTime})
	}
	return items, nil
}

// RecentLogs returns a concise, job-UID-scoped Cloud Logging excerpt.
func RecentLogs(project, jobUID string, limit int) (string, error) {
	if limit < 1 || limit > 50 {
		return "", fmt.Errorf("limit must be between 1 and 50")
	}
	filter := fmt.Sprintf(`logName="projects/%s/logs/batch_task_logs" AND labels.job_uid="%s"`, project, jobUID)
	out, err := exec.Command("gcloud", "logging", "read", filter,
		"--project", project, "--format", "value(textPayload)",
		"--limit", fmt.Sprintf("%d", limit)).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gcloud logging read failed: %w\n%s", err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

// CancelJob cancels one non-terminal Batch job by its exact GARY job ID.
func CancelJob(project, region, jobID string) error {
	out, err := exec.Command("gcloud", "batch", "jobs", "cancel", jobID,
		"--project", project, "--location", region, "--quiet").CombinedOutput()
	if err != nil {
		return fmt.Errorf("gcloud batch cancel failed: %w\n%s", err, string(out))
	}
	return nil
}

// QueryLogs queries Cloud Logging for the heartbeat marker from the given job UID.
// Returns the log text if found, or an error if not found.
func QueryLogs(project, jobUID, heartbeat string) (string, error) {
	filter := fmt.Sprintf(
		`logName="projects/%s/logs/batch_task_logs" AND labels.job_uid="%s"`,
		project, jobUID,
	)

	args := []string{
		"logging", "read", filter,
		"--project", project,
		"--format", "value(textPayload)",
		"--freshness", "10m",
		"--limit", "10",
	}

	out, err := exec.Command("gcloud", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gcloud logging read failed: %s\n%s", err, string(out))
	}

	logs := strings.TrimSpace(string(out))
	if logs == "" {
		return "", fmt.Errorf("no logs found for job UID %s", jobUID)
	}

	if strings.Contains(logs, heartbeat) {
		return logs, nil
	}

	return "", fmt.Errorf("heartbeat %q not found in logs for job %s: %s", heartbeat, jobUID, logs)
}

// WaitForTerminal polls the job until it reaches a terminal state or times out.
// Returns the final JobStatus and the time the job finished.
func WaitForTerminal(project, region, jobID string, pollInterval time.Duration, timeout time.Duration) (*JobStatus, time.Time, error) {
	deadline := time.Now().Add(timeout)
	lastState := ""

	for {
		if time.Now().After(deadline) {
			return nil, time.Time{}, fmt.Errorf("timeout waiting for job %s to complete", jobID)
		}

		status, err := DescribeJob(project, region, jobID)
		if err != nil {
			return nil, time.Time{}, fmt.Errorf("describe job: %w", err)
		}

		if status.State() != lastState {
			fmt.Println(status.State())
			lastState = status.State()
		}

		if IsTerminal(status.State()) {
			return status, time.Now(), nil
		}

		time.Sleep(pollInterval)
	}
}

// Now returns the current time. Extracted for testability.
func Now() time.Time {
	return time.Now()
}

// CodingJobConfig holds the parameters for a Batch coding job.
type CodingJobConfig struct {
	Project      string
	Region       string
	JobName      string
	RepoURL      string
	BaseBranch   string
	GaryBranch   string
	Task         string
	Acceptance   string
	GCSPath      string
	PrevGCSPath  string
	MachineType  string
	Model        string
	GitHubToken  string
	MaxRunDurSec int
}

// generateCodingWorkerScript builds the raw bash script for a coding worker.
// Separated from GenerateCodingJobConfig for testability and prompt transport.
func generateCodingWorkerScript(cfg CodingJobConfig) string {
	parts := []string{
		"#!/bin/bash",
		"set -e",
		"",
		"export GEMINI_CLI_TRUST_WORKSPACE=true",
		"export CLOUDSDK_PYTHON=/usr/local/bin/python",
		"export GEMINI_CLI_SYSTEM_SETTINGS_PATH=/workspace/gemini-system-settings.json",
		"",
		"cat > /workspace/gemini-system-settings.json <<'JSON'",
		"{\"general\":{\"retryFetchErrors\":false,\"maxAttempts\":1,\"plan\":{\"modelRouting\":false}}}",
		"JSON",
		"",
		"echo \"GARY CODING WORKER STARTING\"",
		"echo \"Time: $(date -u +%Y-%m-%dT%H:%M:%SZ)\"",
		"",
		"# Configure short-lived GitHub App auth for private repositories when supplied",
		"if [ -n \"$GARY_GITHUB_TOKEN\" ]; then",
		"  export GIT_CONFIG_COUNT=1",
		"  export GIT_CONFIG_KEY_0=http.https://github.com/.extraheader",
		"  export GIT_CONFIG_VALUE_0=\"Authorization: Basic $(printf 'x-access-token:%s' \"$GARY_GITHUB_TOKEN\" | base64 | tr -d '\\n')\"",
		"fi",
		"",
		"# Clone the repository",
		fmt.Sprintf("echo \"Cloning %s...\"", cfg.RepoURL),
		fmt.Sprintf("git clone %s /workspace/repo", cfg.RepoURL),
		"cd /workspace/repo",
		"",
		"# Set git identity for commits",
		"git config --global user.email \"gary@agent-yard.local\"",
		"git config --global user.name \"GARY Worker\"",
		"",
		"# Checkout base branch and create GARY branch",
		fmt.Sprintf("echo \"Creating branch %s from %s...\"", cfg.GaryBranch, cfg.BaseBranch),
		fmt.Sprintf("git checkout %s", cfg.BaseBranch),
		fmt.Sprintf("git checkout -b %s", cfg.GaryBranch),
		"",
		"# Apply previous attempt patch if available",
		"if [ -n \"$PREV_PATCH_GCS\" ]; then",
		"  echo \"Applying previous patch from $PREV_PATCH_GCS...\"",
		"  (gcloud storage cp $PREV_PATCH_GCS /workspace/prev.patch 2>/dev/null || python3 -m gsutil cp $PREV_PATCH_GCS /workspace/prev.patch 2>/dev/null) && git apply /workspace/prev.patch && echo \"Previous patch applied\" || echo \"No previous patch to apply\"",
		"fi",
		"",
		"# Decode base64 prompt to file (transport-safe, no shell interpolation)",
		"echo \"$GARY_PROMPT_B64\" | base64 -d > /workspace/gary_prompt.txt",
		"",
		"# Run the pinned Gemini CLI in machine-readable streaming mode.",
		": > /workspace/usage.jsonl",
		"echo \"Running Gemini CLI...\"",
		"set +e",
		fmt.Sprintf("cat /workspace/gary_prompt.txt | gemini -m %s --yolo --output-format stream-json 2>&1 | tee /workspace/usage.jsonl", cfg.Model),
		"GEMINI_EXIT=${PIPESTATUS[0]}",
		"set -e",
		"echo \"Gemini CLI exit code: $GEMINI_EXIT\"",
		"",
		"# Run acceptance checks using exit code as truth",
		"echo \"Running acceptance checks...\"",
		": > /workspace/acceptance_output.txt",
		"set +e",
		fmt.Sprintf("%s > /workspace/acceptance_output.txt 2>&1", cfg.Acceptance),
		"ACCEPTANCE_EXIT=$?",
		"set -e",
		"",
		"# Always preserve acceptance_output.txt",
		"cat /workspace/acceptance_output.txt || true",
		"",
		"# Record acceptance result, but preserve patch/evidence before exiting.",
		"if [ $ACCEPTANCE_EXIT -eq 0 ]; then",
		"  echo \"ACCEPTANCE PASSED\" | tee -a /workspace/acceptance_output.txt",
		"else",
		"  echo \"ACCEPTANCE FAILED (exit code $ACCEPTANCE_EXIT)\" | tee -a /workspace/acceptance_output.txt",
		"fi",
		"",
		"# Remove disposable worker artifacts before staging; never publish environments or caches.",
		"find . -type d \\( -name .venv -o -name __pycache__ -o -name .pytest_cache -o -name .mypy_cache -o -name .ruff_cache -o -name .tox -o -name .nox -o -name .swiftpm -o -name .build -o -name DerivedData \\) -prune -exec rm -rf {} + 2>/dev/null || true",
		"find . -type d -name '*.egg-info' -prune -exec rm -rf {} + 2>/dev/null || true",
		"",
		"# Commit changes",
		"echo \"Committing changes...\"",
		"git add -A",
		"git commit -m \"GARY worker result\" || true",
		"",
		"# Generate patch",
		"echo \"Generating patch...\"",
		fmt.Sprintf("git format-patch %s --stdout > /workspace/result.patch", cfg.BaseBranch),
		"",
		"# Encode patch for transport (GCS upload may fail)",
		"echo \"GARY_PATCH_B64_START\"",
		"base64 /workspace/result.patch",
		"echo \"GARY_PATCH_B64_END\"",
		"echo \"GARY_ACCEPTANCE_B64_START\"",
		"base64 /workspace/acceptance_output.txt",
		"echo \"GARY_ACCEPTANCE_B64_END\"",
		"echo \"GARY_USAGE_B64_START\"",
		"base64 /workspace/usage.jsonl",
		"echo \"GARY_USAGE_B64_END\"",
		"",
		"# Upload to GCS (gcloud storage primary, python3 -m gsutil fallback)",
		fmt.Sprintf("echo \"Uploading result to %s...\"", cfg.GCSPath),
		fmt.Sprintf("gcloud storage cp /workspace/result.patch %s/result.patch 2>/dev/null || python3 -m gsutil cp /workspace/result.patch %s/result.patch || true", cfg.GCSPath, cfg.GCSPath),
		fmt.Sprintf("gcloud storage cp /workspace/acceptance_output.txt %s/acceptance_output.txt 2>/dev/null || python3 -m gsutil cp /workspace/acceptance_output.txt %s/acceptance_output.txt || true", cfg.GCSPath, cfg.GCSPath),
		fmt.Sprintf("gcloud storage cp /workspace/usage.jsonl %s/usage.jsonl 2>/dev/null || python3 -m gsutil cp /workspace/usage.jsonl %s/usage.jsonl || true", cfg.GCSPath, cfg.GCSPath),
		"",
		"# Only now return the mechanical acceptance exit code.",
		"if [ $ACCEPTANCE_EXIT -ne 0 ]; then",
		"  exit $ACCEPTANCE_EXIT",
		"fi",
		"",
		"echo \"GARY CODING WORKER COMPLETE\"",
	}

	return strings.Join(parts, "\n")
}

func codingWorkerPrompt(task string) string {
	return fmt.Sprintf(`You are a GARY implementation Worker. Work only in this repository.

WORKER ROLE
===========
%s

UNIVERSAL GARY RULES
====================
%s

COST AND REPOSITORY HYGIENE
============================
- This is a paid, bounded attempt. Work only on the stated slice; do not explore unrelated parts of the repository or repeat completed work.
- Do not create virtual environments, install dependencies, or run broad dependency/build scans unless the task or acceptance command explicitly requires it. Use temporary locations outside the repository for disposable tooling.
- Never commit generated environments, caches, dependency trees, build products, or machine-local files. Before committing, inspect git status and remove disposable artifacts; the patch must contain only intentional source, test, documentation, and configuration changes.

JOB
===
%s

Do not ask questions unless genuinely blocked.`, doctrine.WorkerRoleRules, doctrine.UniversalRules, task)
}

// GenerateCodingJobConfig produces the JSON config for a Batch coding job.
func GenerateCodingJobConfig(cfg CodingJobConfig) ([]byte, error) {
	if cfg.MachineType == "" {
		cfg.MachineType = "e2-small"
	}
	if cfg.Model == "" {
		cfg.Model = "gemini-3.5-flash-lite"
	}
	if cfg.MaxRunDurSec == 0 {
		cfg.MaxRunDurSec = 600
	}

	// Build the full prompt for the worker.
	prompt := codingWorkerPrompt(cfg.Task)

	// Base64 encode prompt for safe shell transport
	promptB64 := base64.StdEncoding.EncodeToString([]byte(prompt))

	// Build script with the base64 prompt injected as env var
	script := generateCodingWorkerScript(cfg)
	// Inject the prompt as an environment variable at the top of the script
	script = "export GARY_PROMPT_B64=\"" + promptB64 + "\"\n" + script

	variables := map[string]string{
		"PREV_PATCH_GCS": cfg.PrevGCSPath,
	}
	if cfg.GitHubToken != "" {
		variables["GARY_GITHUB_TOKEN"] = cfg.GitHubToken
	}

	config := map[string]interface{}{
		"taskGroups": []map[string]interface{}{
			{
				"taskCount":   "1",
				"parallelism": "1",
				"taskSpec": map[string]interface{}{
					"runnables": []map[string]interface{}{
						{
							"container": map[string]interface{}{
								"imageUri":   "europe-west2-docker.pkg.dev/gary-agent-yard/gary-workers/gary-batch-worker:latest",
								"entrypoint": "/bin/bash",
								"commands":   []string{"-c", script},
							},
						},
					},
					"maxRunDuration": fmt.Sprintf("%ds", cfg.MaxRunDurSec),
					"maxRetryCount":  0,
					"environment": map[string]interface{}{
						"variables": variables,
						"secretVariables": map[string]string{
							"GEMINI_API_KEY": "projects/418146339023/secrets/gary-gemini-api-key/versions/latest",
						},
					},
				},
			},
		},
		"allocationPolicy": map[string]interface{}{
			"instances": []map[string]interface{}{
				{
					"policy": map[string]interface{}{
						"machineType":       cfg.MachineType,
						"provisioningModel": "STANDARD",
					},
				},
			},
			"network": map[string]interface{}{
				"networkInterfaces": []map[string]interface{}{
					{
						"network": "global/networks/default",
					},
				},
			},
			"serviceAccount": map[string]interface{}{
				"email": "gary-batch-worker@gary-agent-yard.iam.gserviceaccount.com",
			},
		},
		"labels": map[string]string{
			"gary-build": "3a",
		},
		"logsPolicy": map[string]interface{}{
			"destination": "CLOUD_LOGGING",
		},
	}

	return json.MarshalIndent(config, "", "  ")
}
