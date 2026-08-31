package main

import (
	"testing"
)

func TestDecodeProjectManualNormalizesYardmasterVariants(t *testing.T) {
	digest := "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	text := `{
  "schema": "gary.project-manual.v1",
  "project_id": "demo",
  "objective": "Test durable intake",
  "definition_of_done": "The marker exists",
  "manual_version": "1",
  "source_packet_digest": "` + digest + `",
  "functional_requirements": [{"id":"REQ-001","description":"Add the marker","source_locator":"MASTER PACKET: ## Objective"}],
  "acceptance_criteria": [{"id":"AC-001","command":"grep -q marker README.md","expected_outcome":"returncode 0","source_locator":"MASTER PACKET: ## Acceptance"}]
}`
	manual, err := decodeProjectManual(text, digest)
	if err != nil {
		t.Fatal(err)
	}
	if manual.ManualVersion != 1 || len(manual.FunctionalRequirements) != 1 || manual.FunctionalRequirements[0].Text != "Add the marker" {
		t.Fatalf("manual normalization = %+v", manual)
	}
	if manual.FunctionalRequirements[0].Source.PacketDigest != digest || manual.FunctionalRequirements[0].Source.Locator != "MASTER PACKET: ## Objective" {
		t.Fatalf("requirement source = %+v", manual.FunctionalRequirements[0].Source)
	}
	if manual.AcceptanceCriteria[0].Text != "grep -q marker README.md" || manual.AcceptanceCriteria[0].Source.Locator == "" {
		t.Fatalf("acceptance normalization = %+v", manual.AcceptanceCriteria[0])
	}
}

func TestDecodeProjectManualRejectsTrailingJSON(t *testing.T) {
	if _, err := decodeProjectManual(`{"manual_version":1}{"manual_version":2}`, "sha256:bad"); err == nil {
		t.Fatal("trailing JSON was accepted")
	}
}
