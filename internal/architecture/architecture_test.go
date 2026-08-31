package architecture

import (
	"strings"
	"testing"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/project"
)

func fixture() (project.AcceptanceContract, project.ArchitectureManifest) {
	digest := "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	contract := project.AcceptanceContract{SchemaVersion: 1, ProjectID: "demo", SourcePacketDigest: digest, ContractVersion: 1, ContractDigest: "sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789", Claims: []project.AcceptanceClaim{{ClaimID: "AC-01", Statement: "records persist", Importance: "required", Boundary: "process_and_persistent_store", EvidenceExpectation: "direct_when_available", Status: "OPEN", Source: project.SourceRef{PacketDigest: digest, Locator: "packet#acceptance"}}}}
	manifest := project.ArchitectureManifest{SchemaVersion: 1, ProjectID: "demo", ContractVersion: 1, ContractDigest: contract.ContractDigest, ManifestVersion: 1, Components: []project.ArchitectureComponent{{ID: "persistent-store", Supports: []string{"AC-01"}, JustificationType: "engineering_necessity", FailureIfOmitted: "Records would be lost on restart."}}}
	manifest.ManifestDigest = project.ArchitectureManifestDigest(manifest)
	return contract, manifest
}

func TestLegitimateEngineeringNecessityPasses(t *testing.T) {
	contract, manifest := fixture()
	result := Validate(manifest, contract)
	if len(result.Errors) != 0 {
		t.Fatalf("legitimate architecture rejected: %+v", result)
	}
}

func TestEngineeringNecessityRequiresFailureExplanation(t *testing.T) {
	contract, manifest := fixture()
	manifest.Components[0].FailureIfOmitted = ""
	result := Validate(manifest, contract)
	if len(result.Errors) == 0 || !strings.Contains(result.Errors[0], "failure_if_omitted") {
		t.Fatalf("missing necessity explanation not rejected: %+v", result)
	}
}

func TestRepairExpansionIsWarned(t *testing.T) {
	contract, previous := fixture()
	current := previous
	current.ManifestVersion = 2
	current.Components = append(current.Components, project.ArchitectureComponent{ID: "redis-cache", Supports: []string{"AC-01"}, JustificationType: "verification_support"})
	current.ManifestDigest = project.ArchitectureManifestDigest(current)
	result := Compare(previous, current)
	if len(result.Warnings) == 0 || !strings.Contains(result.Warnings[0], "redis-cache") {
		t.Fatalf("scope expansion was not warned: %+v", result)
	}
	_ = contract
}
