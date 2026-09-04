package batch

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestGenerateSmokeJobConfig(t *testing.T) {
	data, err := GenerateJobConfig(JobConfig{
		Project: "p", Region: "europe-west2", JobName: "smoke",
		Heartbeat: "GARY_HEARTBEAT", MachineType: "e2-small", MaxRunDurSec: 300,
	})
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	tg := cfg["taskGroups"].([]interface{})[0].(map[string]interface{})
	if tg["taskCount"] != "1" || tg["parallelism"] != "1" {
		t.Fatalf("smoke job must remain one isolated task: %#v", tg)
	}
}

func TestTerminalStates(t *testing.T) {
	for state, want := range map[string]bool{
		"SUCCEEDED": true, "FAILED": true, "CANCELLED": true,
		"RUNNING": false, "QUEUED": false, "": false,
	} {
		if got := IsTerminal(state); got != want {
			t.Fatalf("IsTerminal(%q)=%v want %v", state, got, want)
		}
	}
}

func TestObservedRuntimeSeconds(t *testing.T) {
	status := &JobStatus{CreateTime: "2026-09-04T05:00:00Z", UpdateTime: "2026-09-04T05:02:30Z"}
	if got := ObservedRuntimeSeconds(status, time.Time{}, time.Time{}); got != 150 {
		t.Fatalf("provider runtime %.0f want 150", got)
	}
}

func TestCodingWorkerIsOneAutonomousAgent(t *testing.T) {
	script := generateCodingWorkerScript(CodingJobConfig{
		RepoURL: "https://example.com/repo.git", BaseBranch: "main", GaryBranch: "gary/test",
		Task: "finish the whole job", Acceptance: "go test ./...", GCSPath: "gs://bucket/run",
		Model: "gemini-3.8-flash",
	})
	for _, required := range []string{
		"GARY SINGLE AGENT STARTING",
		"gemini -m gemini-3.8-flash --yolo --output-format stream-json",
		"\"modelRouting\":false",
		"\"maxAttempts\":1",
		".gary-blocked.json",
		"GARY_BLOCKER_B64_START",
		"ACCEPTANCE_EXIT=$?",
		"git format-patch",
		"GARY SINGLE AGENT COMPLETE",
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("single-agent worker missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"emit_telemetry", "apply_running_control", "STOP_AND_REPLAN",
		"READY_FOR_INSPECTION", "--resume", "Waiting for Yardmaster review",
		"checkpoint_candidate", "pending_guidance",
	} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("single-agent worker retained obsolete supervisor machinery %q", forbidden)
		}
	}
}

func TestCodingWorkerKeepsAcceptanceIndependentOfBatchState(t *testing.T) {
	script := generateCodingWorkerScript(CodingJobConfig{
		RepoURL: "https://example.com/repo.git", BaseBranch: "main", GaryBranch: "gary/test",
		Task: "finish it", Acceptance: "go test ./...", GCSPath: "gs://bucket/run",
	})
	if strings.Contains(script, "grep -q") {
		t.Fatal("acceptance must use exit code, not output text")
	}
	for _, marker := range []string{"ACCEPTANCE_EXIT=$?", "$ACCEPTANCE_EXIT -eq 0", "exit 0"} {
		if !strings.Contains(script, marker) {
			t.Fatalf("worker missing acceptance boundary %q", marker)
		}
	}
}

func TestCodingJobPassesShortLivedTokenWithoutAppPrivateKey(t *testing.T) {
	data, err := GenerateCodingJobConfig(CodingJobConfig{
		Project: "p", Region: "r", JobName: "j",
		RepoURL: "https://github.com/example/private.git", BaseBranch: "main",
		GaryBranch: "gary/test", Task: "whole job", Acceptance: "echo ok",
		GitHubToken: "installation-token", Model: "gemini-3.8-flash",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "GARY_GITHUB_TOKEN") || !strings.Contains(text, "installation-token") {
		t.Fatal("short-lived repository token missing")
	}
	for _, forbidden := range []string{"GARY_GITHUB_APP_PRIVATE_KEY", "BEGIN PRIVATE KEY", "BEGIN RSA PRIVATE KEY"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("worker config leaked control-plane credential material %q", forbidden)
		}
	}
}

func TestCodingJobTransportDoesNotExposeRawTask(t *testing.T) {
	task := "special $TASK with `shell` and {\"json\":true}"
	data, err := GenerateCodingJobConfig(CodingJobConfig{
		Project: "p", Region: "r", JobName: "j", RepoURL: "https://example.com/repo.git",
		BaseBranch: "main", GaryBranch: "gary/test", Task: task, Acceptance: "echo ok",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), task) {
		t.Fatal("raw task must be base64 transported rather than interpolated into shell/JSON")
	}
}
