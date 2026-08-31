package acceptance

import (
	"strings"
	"testing"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/architecture"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/project"
)

func evaluationFixture(boundary, result, evidence string) (project.AcceptanceContract, project.InspectionLedger) {
	digest := "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	claim := project.AcceptanceClaim{ClaimID: "AC-01", Statement: "the capability works", Importance: "required", Boundary: boundary, EvidenceExpectation: "direct_when_available", Status: "OPEN", Source: project.SourceRef{PacketDigest: digest, Locator: "packet#acceptance"}}
	contract := project.AcceptanceContract{SchemaVersion: 1, ProjectID: "fixture", SourcePacketDigest: digest, ContractVersion: 1, ContractDigest: "sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789", Claims: []project.AcceptanceClaim{claim}}
	entry := project.InspectionEntry{ClaimID: claim.ClaimID, Claim: claim.Statement, Boundary: claim.Boundary, InspectionSeam: "fixture-seam", VectorOrAction: "adversarial vector", EvidenceType: evidence, Result: result, InspectorRunID: "inspector-fixture", Timestamp: "now", FailureReason: "fixture failure"}
	return contract, project.InspectionLedger{SchemaVersion: 1, ProjectID: contract.ProjectID, ContractVersion: 1, ContractDigest: contract.ContractDigest, InspectedCommitSHA: "abc1234", InspectorRunID: "inspector-fixture", Timestamp: "now", Entries: []project.InspectionEntry{entry}}
}

func TestTelltailStyleAcceptanceRegressionMatrix(t *testing.T) {
	tests := []struct {
		name, boundary, result, evidence, want string
	}{
		{"known-good", "application", project.InspectionPass, "unit", "PASS"},
		{"proxy-only", "real_external_service", project.InspectionPass, "synthetic", "FAIL"},
		{"compile-failure", "application", project.InspectionFail, "static", "FAIL"},
		{"persistence-fake", "process_and_persistent_store", project.InspectionPass, "unit", "FAIL"},
		{"hardware-unavailable", "physical_device", project.InspectionBlockedEnvironment, "static", "ACCEPTED_WITH_BLOCKED_VERIFICATION"},
		{"fast-lane", "application", project.InspectionPass, "static", "PASS"},
		{"mock-string-false-positive", "application", project.InspectionPass, "unit", "PASS"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			contract, ledger := evaluationFixture(tc.boundary, tc.result, tc.evidence)
			got := Evaluate(contract, ledger, true)
			if got.Status != tc.want {
				t.Fatalf("status=%s failures=%v blocked=%v want=%s", got.Status, got.Failures, got.Blocked, tc.want)
			}
		})
	}
}

func TestArchitectureScopeAndContractMutationRegression(t *testing.T) {
	digest := "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	contract := project.AcceptanceContract{SchemaVersion: 1, ProjectID: "fixture", SourcePacketDigest: digest, ContractVersion: 1, ContractDigest: "sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789", Claims: []project.AcceptanceClaim{{ClaimID: "AC-01", Statement: "records persist", Importance: "required", Boundary: "process_and_persistent_store", EvidenceExpectation: "direct_when_available", Status: "OPEN", Source: project.SourceRef{PacketDigest: digest, Locator: "packet#acceptance"}}}}
	manifest := project.ArchitectureManifest{SchemaVersion: 1, ProjectID: "fixture", ContractVersion: 1, ContractDigest: contract.ContractDigest, ManifestVersion: 1, Components: []project.ArchitectureComponent{{ID: "redis-cache", Supports: []string{"AC-01"}, JustificationType: "verification_support"}}}
	manifest.ManifestDigest = project.ArchitectureManifestDigest(manifest)
	if result := architecture.Validate(manifest, contract); len(result.Warnings) == 0 || !strings.Contains(strings.Join(result.Warnings, "\n"), "external infrastructure") {
		t.Fatalf("scope creep was not warned: %+v", result)
	}
	ledger := project.InspectionLedger{SchemaVersion: 1, ProjectID: "fixture", ContractVersion: 1, ContractDigest: "sha256:wrong", InspectedCommitSHA: "abc1234", InspectorRunID: "inspector-fixture", Timestamp: "now", Entries: []project.InspectionEntry{{ClaimID: "AC-01", Claim: "records persist", Boundary: "process_and_persistent_store", InspectionSeam: "restart", VectorOrAction: "reopen", EvidenceType: "component_real", Result: project.InspectionPass, InspectorRunID: "inspector-fixture", Timestamp: "now"}}}
	if err := project.ValidateInspectionLedger(ledger, contract, "abc1234"); err == nil {
		t.Fatal("acceptance mutation via contract digest was accepted")
	}
}
