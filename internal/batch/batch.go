// Package batch isolates Google Cloud Batch submission and status queries.
// It uses os/exec to call documented gcloud CLI commands.
package batch

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
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
type JobStatus struct {
	UID   string `json:"uid"`
	State string `json:"state"`
	Name  string `json:"name"`
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
		"--location=" + cfg.Region,
		"--project=" + cfg.Project,
		"--config=" + tmpFile.Name(),
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
		"--location=" + region,
		"--project=" + project,
		"--format=json",
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

// QueryLogs queries Cloud Logging for the heartbeat marker from the given job UID.
// Returns the log text if found, or an error if not found.
func QueryLogs(project, jobUID, heartbeat string) (string, error) {
	filter := fmt.Sprintf(
		`logName="projects/%s/logs/batch_task_logs" AND labels.job_uid="%s"`,
		project, jobUID,
	)

	args := []string{
		"logging", "read", filter,
		"--project=" + project,
		"--format=value(textPayload)",
		"--freshness=10m",
		"--limit=10",
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

		if status.State != lastState {
			fmt.Println(status.State)
			lastState = status.State
		}

		if IsTerminal(status.State) {
			return status, time.Now(), nil
		}

		time.Sleep(pollInterval)
	}
}

// Now returns the current time. Extracted for testability.
func Now() time.Time {
	return time.Now()
}
