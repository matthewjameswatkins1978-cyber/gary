// Package batch is GARY's narrow Google Cloud Batch boundary.
package batch

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/doctrine"
)

const (
	gcloudCommandTimeout    = 45 * time.Second
	gcloudSubmissionTimeout = 2 * time.Minute
)

func runGcloud(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gcloudCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gcloud", args...)
	cmd.WaitDelay = 2 * time.Second
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return out, fmt.Errorf("gcloud command timed out after %s: %w", gcloudCommandTimeout, ctx.Err())
	}
	return out, err
}

// JobConfig describes the cheap deterministic Batch heartbeat smoke test.
type JobConfig struct {
	Project      string
	Region       string
	JobName      string
	Heartbeat    string
	MachineType  string
	MaxRunDurSec int
	MaxRetries   int
}

// JobStatus is the small subset of Google Batch status GARY relies on.
type JobStatus struct {
	UID        string        `json:"uid"`
	Name       string        `json:"name"`
	CreateTime string        `json:"createTime"`
	UpdateTime string        `json:"updateTime"`
	Status     JobStatusInfo `json:"status"`
}

type JobStatusInfo struct {
	State string `json:"state"`
}

func (js *JobStatus) State() string {
	if js == nil {
		return ""
	}
	return js.Status.State
}

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

var terminalStates = map[string]bool{
	"SUCCEEDED": true,
	"FAILED":    true,
	"CANCELLED": true,
	"DELETING":  true,
	"DELETED":   true,
}

func IsTerminal(state string) bool { return terminalStates[state] }

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

func GenerateJobConfig(cfg JobConfig) ([]byte, error) {
	if cfg.MachineType == "" {
		cfg.MachineType = "e2-small"
	}
	if cfg.MaxRunDurSec == 0 {
		cfg.MaxRunDurSec = 300
	}
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 0
	}

	script := "#!/bin/sh\necho \"" + cfg.Heartbeat + "\"\nexit 0\n"
	config := map[string]interface{}{
		"taskGroups": []map[string]interface{}{{
			"taskCount":   "1",
			"parallelism": "1",
			"taskSpec": map[string]interface{}{
				"runnables": []map[string]interface{}{{"script": map[string]interface{}{"text": script}}},
				"maxRunDuration": fmt.Sprintf("%ds", cfg.MaxRunDurSec),
				"maxRetryCount":  cfg.MaxRetries,
			},
		}},
		"allocationPolicy": map[string]interface{}{
			"instances": []map[string]interface{}{{"policy": map[string]interface{}{
				"machineType": cfg.MachineType, "provisioningModel": "STANDARD",
			}}},
		},
		"labels":     map[string]string{"gary": "smoke"},
		"logsPolicy": map[string]interface{}{"destination": "CLOUD_LOGGING"},
	}
	return json.MarshalIndent(config, "", "  ")
}

func SubmitJob(cfg JobConfig) (string, error) {
	data, err := GenerateJobConfig(cfg)
	if err != nil {
		return "", err
	}
	return submitConfig(cfg.Project, cfg.Region, cfg.JobName, data, "gary-batch-smoke-*.json")
}

// CodingJobConfig describes one isolated whole-job agent invocation.
type CodingJobConfig struct {
	Project          string
	Region           string
	JobName          string
	RepoURL          string
	BaseBranch       string
	GaryBranch       string
	Task             string
	Acceptance       string
	EstimatedMinutes int
	GCSPath          string
	MachineType      string
	Model            string
	GitHubToken      string
	MaxRunDurSec     int
}

func generateCodingWorkerScript(cfg CodingJobConfig) string {
	return singleAgentCodingWorkerScript(cfg)
}

func codingWorkerPrompt(task, acceptance string, _ int) string {
	return fmt.Sprintf(`You are GARY, the single implementation agent responsible for the complete supplied job.

COMPLETION PRINCIPLE
====================
%s

ROLE
====
%s

CORE RULES
==========
%s

EXECUTION CONTRACT
==================
- Own the supplied job end to end. There is no Yardmaster, child-worker hierarchy, live supervisor, or automatic stronger model behind you.
- Inspect the relevant repository and environment before significant edits. Use tools and direct evidence rather than guessing.
- Obtain ordinary installable tools, packages, SDKs and documentation yourself when useful and within the job's authority.
- Preserve useful work when an approach fails. Change course from evidence rather than repeating the same failed move.
- Keep the solution simple, maintainable and idiomatic. Do not expand into unrelated redesign.
- Run the relevant checks and leave only intentional repository changes.
- If, and only if, a genuine human/authority boundary prevents completion, create .gary-blocked.json at the repository root with this exact shape:
  {"kind":"authority|credential|spend|destructive_action|product_decision|model_escalation|external","summary":"...","needed":"...","evidence":"...","unblock_action":"..."}
  Do not use a blocker for a missing ordinary tool, first failure, ordinary merge conflict, slow build, or a route that simply needs rethinking.
- Model escalation is never automatic. If the assigned model is genuinely the limiting factor, request it through the structured blocker with concrete evidence.
- Finish when the deliverable exists and the requested checks are satisfied. Do not keep polishing after completion.

JOB
===
%s

INDEPENDENT CHECKS
==================
GARY will run these checks outside your model process against the returned candidate:
%s

Use your judgement to finish the actual job.`, doctrine.CompletionPrinciple, doctrine.WorkerRoleRules, doctrine.UniversalRules, task, acceptance)
}

func GenerateCodingJobConfig(cfg CodingJobConfig) ([]byte, error) {
	if cfg.MachineType == "" {
		cfg.MachineType = "e2-small"
	}
	if cfg.Model == "" {
		cfg.Model = "gemini-3.8-flash"
	}
	if cfg.MaxRunDurSec == 0 {
		cfg.MaxRunDurSec = 3600
	}
	if strings.TrimSpace(cfg.RepoURL) == "" || strings.TrimSpace(cfg.BaseBranch) == "" || strings.TrimSpace(cfg.GaryBranch) == "" {
		return nil, fmt.Errorf("repo, base branch and result branch are required")
	}

	promptB64 := base64.StdEncoding.EncodeToString([]byte(codingWorkerPrompt(cfg.Task, cfg.Acceptance, cfg.EstimatedMinutes)))
	script := "export GARY_PROMPT_B64=\"" + promptB64 + "\"\n" + generateCodingWorkerScript(cfg)
	variables := map[string]string{
		"GARY_GCS_PATH":    cfg.GCSPath,
		"GARY_JOB_ID":      cfg.JobName,
		"GARY_BASE_BRANCH": cfg.BaseBranch,
	}
	if cfg.GitHubToken != "" {
		variables["GARY_GITHUB_TOKEN"] = cfg.GitHubToken
	}

	config := map[string]interface{}{
		"taskGroups": []map[string]interface{}{{
			"taskCount":   "1",
			"parallelism": "1",
			"taskSpec": map[string]interface{}{
				"runnables": []map[string]interface{}{{"container": map[string]interface{}{
					"imageUri":   "europe-west2-docker.pkg.dev/gary-agent-yard/gary-workers/gary-batch-worker:latest",
					"entrypoint": "/bin/bash",
					"commands":   []string{"-c", script},
				}}},
				"maxRunDuration": fmt.Sprintf("%ds", cfg.MaxRunDurSec),
				"maxRetryCount":  0,
				"environment": map[string]interface{}{
					"variables": variables,
					"secretVariables": map[string]string{
						"GEMINI_API_KEY": "projects/418146339023/secrets/gary-gemini-api-key/versions/latest",
					},
				},
			},
		}},
		"allocationPolicy": map[string]interface{}{
			"instances": []map[string]interface{}{{"policy": map[string]interface{}{
				"machineType": cfg.MachineType, "provisioningModel": "STANDARD",
			}}},
			"network": map[string]interface{}{"networkInterfaces": []map[string]interface{}{{
				"network": "global/networks/default",
			}}},
			"serviceAccount": map[string]interface{}{
				"email": "gary-batch-worker@gary-agent-yard.iam.gserviceaccount.com",
			},
		},
		"labels":     map[string]string{"gary": "single-agent"},
		"logsPolicy": map[string]interface{}{"destination": "CLOUD_LOGGING"},
	}
	return json.MarshalIndent(config, "", "  ")
}

func SubmitCodingJob(cfg CodingJobConfig) (string, error) {
	data, err := GenerateCodingJobConfig(cfg)
	if err != nil {
		return "", fmt.Errorf("generate coding config: %w", err)
	}
	return submitConfig(cfg.Project, cfg.Region, cfg.JobName, data, "gary-batch-coding-*.json")
}

func submitConfig(project, region, jobName string, data []byte, pattern string) (string, error) {
	tmp, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", fmt.Errorf("create Batch config: %w", err)
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", fmt.Errorf("write Batch config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close Batch config: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), gcloudSubmissionTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gcloud", "batch", "jobs", "submit", jobName,
		"--location", region, "--project", project, "--config", name, "--quiet")
	cmd.WaitDelay = 2 * time.Second
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return "", fmt.Errorf("gcloud batch submit timed out after %s: %w", gcloudSubmissionTimeout, ctx.Err())
	}
	if err != nil {
		return "", fmt.Errorf("gcloud batch submit failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return jobName, nil
}

func DescribeJob(project, region, jobID string) (*JobStatus, error) {
	out, err := runGcloud("batch", "jobs", "describe", jobID,
		"--location", region, "--project", project, "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("gcloud batch describe failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	var status JobStatus
	if err := json.Unmarshal(out, &status); err != nil {
		return nil, fmt.Errorf("parse Batch status: %w", err)
	}
	return &status, nil
}

func WaitForTerminal(project, region, jobID string, pollInterval, timeout time.Duration) (*JobStatus, time.Time, error) {
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return nil, time.Time{}, fmt.Errorf("timeout waiting for job %s", jobID)
		}
		status, err := DescribeJob(project, region, jobID)
		if err != nil {
			return nil, time.Time{}, err
		}
		if IsTerminal(status.State()) {
			return status, time.Now(), nil
		}
		time.Sleep(pollInterval)
	}
}

func QueryLogs(project, jobUID, heartbeat string) (string, error) {
	filter := fmt.Sprintf(`logName="projects/%s/logs/batch_task_logs" AND labels.job_uid="%s"`, project, jobUID)
	out, err := runGcloud("logging", "read", filter,
		"--project", project, "--format", "value(textPayload)", "--freshness", "10m", "--limit", "20")
	if err != nil {
		return "", fmt.Errorf("gcloud logging read failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	logs := strings.TrimSpace(string(out))
	if logs == "" || !strings.Contains(logs, heartbeat) {
		return "", fmt.Errorf("heartbeat %q not found for Batch job %s", heartbeat, jobUID)
	}
	return logs, nil
}
