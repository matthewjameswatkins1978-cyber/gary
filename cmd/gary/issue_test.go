package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validIssueEvent(t *testing.T, command string) string {
	t.Helper()
	payload := map[string]any{
		"action":     "opened",
		"repository": map[string]any{"full_name": trustedGaryRepo, "id": 1344689742},
		"sender":     map[string]any{"login": trustedGitHubUser, "id": trustedGitHubUserID},
		"issue": map[string]any{
			"number": 42,
			"title":  issueCommandTitle,
			"body":   command,
			"user":   map[string]any{"login": trustedGitHubUser, "id": trustedGitHubUserID},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "event.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadIssueCommandAcceptsTrustedStatus(t *testing.T) {
	path := validIssueEvent(t, `{"version":1,"command":"status"}`)
	event, command, err := loadIssueCommand(path)
	if err != nil {
		t.Fatalf("loadIssueCommand: %v", err)
	}
	if event.Issue.Number != 42 || command.Command != "status" {
		t.Fatalf("unexpected parsed command: %#v", command)
	}
}

func TestLoadIssueCommandRejectsAuthorBoundary(t *testing.T) {
	path := validIssueEvent(t, `{"version":1,"command":"status"}`)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), trustedGitHubUser, "attacker", 1))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadIssueCommand(path); err == nil || !strings.Contains(err.Error(), "trusted GARY operator") {
		t.Fatalf("expected author rejection, got %v", err)
	}
}

func TestLoadIssueCommandRejectsUnknownJSONAndShellAcceptance(t *testing.T) {
	unknown := validIssueEvent(t, `{"version":1,"command":"status","surprise":true}`)
	if _, _, err := loadIssueCommand(unknown); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected strict JSON rejection, got %v", err)
	}
	shell := validIssueEvent(t, `{"version":1,"command":"submit","job":{"id":"safe-job","repo":"https://github.com/matthewjameswatkins1978-cyber/gary.git","base_branch":"master","task":"Make a bounded change.","acceptance":["sh_c_anything"]}}`)
	if _, _, err := loadIssueCommand(shell); err == nil || !strings.Contains(err.Error(), "not approved") {
		t.Fatalf("expected acceptance profile rejection, got %v", err)
	}
}

func TestLoadIssueCommandProtectsJC01AndJC02(t *testing.T) {
	path := validIssueEvent(t, `{"version":1,"command":"submit","job":{"id":"jc-02","repo":"https://github.com/matthewjameswatkins1978-cyber/gary.git","base_branch":"master","task":"Do not run.","acceptance":["go_test_all"]}}`)
	if _, _, err := loadIssueCommand(path); err == nil || !strings.Contains(err.Error(), "protected") {
		t.Fatalf("expected protected job rejection, got %v", err)
	}
	if !protectedJob("gary-run-jc-01-1-20260825-025411") || protectedJob("gary-run-jc-03-1-20260825-025411") {
		t.Fatal("protected job matcher does not preserve the JC boundary")
	}
}

func TestIssueWorkflowHasOpenOnlyIdempotencyGuard(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "gary-control.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(data)
	for _, required := range []string{"issues:", "types: [opened]", "Refuse a repeated command", "gary-control-issue:${context.issue.number}", "already_processed"} {
		if !strings.Contains(workflow, required) {
			t.Errorf("workflow lacks idempotency/auth requirement %q", required)
		}
	}
}
