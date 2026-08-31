package acceptance

import (
	"strings"
	"testing"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/project"
)

func TestRepairPacketContainsOnlyFailedClaims(t *testing.T) {
	digest := "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	contract := project.AcceptanceContract{ProjectID: "demo", ContractVersion: 1, ContractDigest: "sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789", Claims: []project.AcceptanceClaim{{ClaimID: "AC-01", Statement: "real provider works", Importance: "required"}, {ClaimID: "AC-02", Statement: "records persist", Importance: "required"}}, SourcePacketDigest: digest}
	ledger := project.InspectionLedger{ProjectID: "demo", ContractVersion: 1, ContractDigest: contract.ContractDigest, InspectedCommitSHA: "abc1234", InspectorRunID: "inspect-1", Timestamp: "now", Entries: []project.InspectionEntry{{ClaimID: "AC-01", Claim: "real provider works", Boundary: "real_external_service", InspectionSeam: "probe", VectorOrAction: "obscure ISBN", EvidenceArtifact: "evidence/ac-01.json", EvidenceType: "synthetic", Result: project.InspectionFail, FailureReason: "synthetic evidence only", InspectorRunID: "inspect-1", Timestamp: "now"}, {ClaimID: "AC-02", Claim: "records persist", Boundary: "process_and_persistent_store", InspectionSeam: "restart", VectorOrAction: "reopen", EvidenceType: "component_real", Result: project.InspectionPass, InspectorRunID: "inspect-1", Timestamp: "now"}}}
	packet, err := BuildRepairPacket(contract, ledger)
	if err != nil {
		t.Fatal(err)
	}
	if len(packet.FailedClaims) != 1 || packet.FailedClaims[0].ClaimID != "AC-01" || packet.PacketDigest == "" {
		t.Fatalf("repair packet = %+v", packet)
	}
	task := RepairTask(packet)
	if !strings.Contains(task, "AC-01") || strings.Contains(task, "AC-02") || !strings.Contains(task, "do not edit or regenerate") {
		t.Fatalf("repair task was not narrow/protected: %s", task)
	}
}
