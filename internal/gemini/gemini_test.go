package gemini

import (
	"encoding/json"
	"testing"
)

func TestInteractionRequestJSON(t *testing.T) {
	req := InteractionRequest{
		Agent: "antigravity-preview-05-2026",
		Input: "Say OK",
		Environment: &Environment{
			Type: "remote",
		},
		Tools: []Tool{
			{Type: "bash"},
			{Type: "filesystem"},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got InteractionRequest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Agent != "antigravity-preview-05-2026" {
		t.Errorf("Agent = %q, want antigravity-preview-05-2026", got.Agent)
	}
	if got.Input != "Say OK" {
		t.Errorf("Input = %q, want Say OK", got.Input)
	}
	if got.Environment == nil || got.Environment.Type != "remote" {
		t.Errorf("Environment.Type = %v, want remote", got.Environment)
	}
	if len(got.Tools) != 2 {
		t.Errorf("len(Tools) = %d, want 2", len(got.Tools))
	}
}

func TestInteractionResponseJSON(t *testing.T) {
	respJSON := `{
		"id": "test-id-123",
		"status": "completed",
		"usage": {
			"total_tokens": 1000,
			"total_input_tokens": 500,
			"total_output_tokens": 100,
			"total_thought_tokens": 400,
			"total_cached_tokens": 0,
			"total_tool_use_tokens": 0
		},
		"environment_id": "env-456",
		"service_tier": "standard",
		"model": "antigravity-preview-05-2026",
		"agent": "antigravity-preview-05-2026"
	}`

	var resp InteractionResponse
	if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.ID != "test-id-123" {
		t.Errorf("ID = %q, want test-id-123", resp.ID)
	}
	if resp.Status != "completed" {
		t.Errorf("Status = %q, want completed", resp.Status)
	}
	if resp.Usage == nil {
		t.Fatal("Usage is nil")
	}
	if resp.Usage.TotalTokens != 1000 {
		t.Errorf("Usage.TotalTokens = %d, want 1000", resp.Usage.TotalTokens)
	}
	if resp.EnvironmentID != "env-456" {
		t.Errorf("EnvironmentID = %q, want env-456", resp.EnvironmentID)
	}
}

func TestEnvironmentInfoJSON(t *testing.T) {
	envJSON := `{
		"id": "env-789",
		"status": "active",
		"file_count": "14157",
		"size_bytes": "252224168"
	}`

	var info EnvironmentInfo
	if err := json.Unmarshal([]byte(envJSON), &info); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if info.ID != "env-789" {
		t.Errorf("ID = %q, want env-789", info.ID)
	}
	if info.Status != "active" {
		t.Errorf("Status = %q, want active", info.Status)
	}
	if info.FileCount != "14157" {
		t.Errorf("FileCount = %q, want 14157", info.FileCount)
	}
}
