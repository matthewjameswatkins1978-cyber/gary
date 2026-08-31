package inspector

import (
	"strings"
	"testing"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/project"
)

func testContext() project.ProjectContext {
	p := project.Project{ProjectID: "demo", MasterPacketDigest: "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}
	c := project.AcceptanceContract{
		SchemaVersion: 1, ProjectID: p.ProjectID, SourcePacketDigest: p.MasterPacketDigest,
		ContractVersion: 1, ContractDigest: "sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		Claims: []project.AcceptanceClaim{{ClaimID: "AC-01", Statement: "Records survive restart", Importance: "required", Boundary: "process_and_persistent_store", EvidenceExpectation: "direct_when_available", Status: "OPEN", Source: project.SourceRef{PacketDigest: p.MasterPacketDigest, Locator: "packet#acceptance"}}},
	}
	return project.ProjectContext{Project: p, Manual: project.ProjectManual{ProjectID: p.ProjectID}, Contract: c}
}

func TestBuildPromptContainsOnlyInspectorInputs(t *testing.T) {
	prompt := BuildPrompt(PromptInput{Context: testContext(), InspectedCommitSHA: "abc1234", InspectorRunID: "inspect-1", RepositorySnapshot: "README.md", BuilderClaims: []string{"worker says ready"}})
	for _, required := range []string{"Independent Inspector", "Records survive restart", "worker says ready", "README.md", "fresh context"} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("prompt missing %q", required)
		}
	}
	for _, forbidden := range []string{"repairGuidance", "private worker notes", "hidden decision text", "CLOSER TASK"} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("prompt leaked forbidden context %q", forbidden)
		}
	}
}

func TestDecodeResponseBindsImmutableProvenanceAndCoverage(t *testing.T) {
	ctx := testContext()
	response := `{"entries":[{"claim_id":"AC-01","claim":"Records survive restart","boundary":"process_and_persistent_store","inspection_seam":"restart-probe","vector_or_action":"write, terminate, relaunch, query","evidence_type":"component_real","result":"PASS","inspector_run_id":"spoof","timestamp":""}]}`
	ledger, err := DecodeResponse(response, ctx, "deadbeef", "inspect-1", true)
	if err != nil {
		t.Fatal(err)
	}
	if ledger.ProjectID != "demo" || ledger.ContractDigest != ctx.Contract.ContractDigest || ledger.InspectedCommitSHA != "deadbeef" || ledger.InspectorRunID != "inspect-1" || ledger.Entries[0].InspectorRunID != "inspect-1" || ledger.LedgerDigest == "" {
		t.Fatalf("provenance not bound: %+v", ledger)
	}
}

func TestDecodeResponseRejectsMissingRequiredClaim(t *testing.T) {
	ctx := testContext()
	_, err := DecodeResponse(`{"entries":[]}`, ctx, "deadbeef", "inspect-1", true)
	if err == nil {
		t.Fatal("empty Inspector ledger accepted")
	}
}

func TestEvidenceParityDoesNotAcceptSyntheticRealWorldClaims(t *testing.T) {
	physical := project.AcceptanceClaim{Boundary: "physical_device"}
	if EvidenceParity(physical, "synthetic", true) {
		t.Fatal("synthetic evidence closed a physical claim")
	}
	realService := project.AcceptanceClaim{Boundary: "real_external_service"}
	if EvidenceParity(realService, "synthetic", true) {
		t.Fatal("synthetic evidence closed an available real-service claim")
	}
	if !EvidenceParity(realService, "synthetic", false) {
		t.Fatal("synthetic evidence should remain usable when direct evidence is unavailable")
	}
}
