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
  "definition_of_done": [{"id":"DONE-001","description":"The marker exists"}],
  "manual_version": "1.0.0",
  "source_packet_digest": "` + digest + `",
  "forbidden_changes": ["Do not add dependencies"],
  "important_rationale": "Keep the certification bounded",
  "functional_requirements": [{"id":"REQ-001","description":"Add the marker","source_locator":"MASTER PACKET: ## Objective"}],
  "acceptance_criteria": [{"id":"AC-001","command":"grep -q marker README.md","expected_outcome":"returncode 0","source_locator":"MASTER PACKET: ## Acceptance"}]
}`
	manual, err := decodeProjectManual(text, digest)
	if err != nil {
		t.Fatal(err)
	}
	if manual.ManualVersion != 1 || manual.DefinitionOfDone != "The marker exists" || len(manual.FunctionalRequirements) != 1 || manual.FunctionalRequirements[0].Text != "Add the marker" {
		t.Fatalf("manual normalization = %+v", manual)
	}
	if manual.FunctionalRequirements[0].Source.PacketDigest != digest || manual.FunctionalRequirements[0].Source.Locator != "MASTER PACKET: ## Objective" {
		t.Fatalf("requirement source = %+v", manual.FunctionalRequirements[0].Source)
	}
	if len(manual.ForbiddenChanges) != 1 || manual.ForbiddenChanges[0].Text != "Do not add dependencies" {
		t.Fatalf("string requirement normalization = %+v", manual.ForbiddenChanges)
	}
	if len(manual.ImportantRationale) != 1 || manual.ImportantRationale[0].Text != "Keep the certification bounded" {
		t.Fatalf("scalar list normalization = %+v", manual.ImportantRationale)
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

func TestDecodeProjectManualRepairsBlankEquivalentFields(t *testing.T) {
	digest := "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	text := `{
  "schema": "gary.project-manual.v1",
  "project_id": "demo",
  "objective": "Test durable intake",
  "definition_of_done": "The marker exists",
  "manual_version": 1,
  "source_packet_digest": "` + digest + `",
  "functional_requirements": [{
    "id": "",
    "text": "",
    "description": "Add the marker",
    "source": {"packet_digest": "", "locator": ""}
  }]
}`
	manual, err := decodeProjectManual(text, digest)
	if err != nil {
		t.Fatal(err)
	}
	if len(manual.FunctionalRequirements) != 1 {
		t.Fatalf("requirements = %+v", manual.FunctionalRequirements)
	}
	requirement := manual.FunctionalRequirements[0]
	if requirement.ID != "functional_requirements-001" || requirement.Text != "Add the marker" {
		t.Fatalf("requirement normalization = %+v", requirement)
	}
	if requirement.Source.PacketDigest != digest || requirement.Source.Locator != "MASTER_PACKET.md#functional_requirements" {
		t.Fatalf("requirement source = %+v", requirement.Source)
	}
}

