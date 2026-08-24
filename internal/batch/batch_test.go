package batch

import (
	"encoding/json"
	"strings"
	"testing"
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
