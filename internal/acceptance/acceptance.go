// Package acceptance translates Inspector evidence into narrow repair work.
package acceptance

import (
	"fmt"
	"strings"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/project"
)

func BuildRepairPacket(contract project.AcceptanceContract, ledger project.InspectionLedger) (project.RepairPacket, error) {
	claims := map[string]project.AcceptanceClaim{}
	for _, claim := range contract.Claims {
		claims[claim.ClaimID] = claim
	}
	packet := project.RepairPacket{
		SchemaVersion: 1, ProjectID: contract.ProjectID, ContractVersion: contract.ContractVersion,
		ContractDigest: contract.ContractDigest, InspectedCommitSHA: ledger.InspectedCommitSHA,
		InspectorRunID: ledger.InspectorRunID, CreatedAt: ledger.Timestamp,
	}
	for _, entry := range ledger.Entries {
		if entry.Result != project.InspectionFail {
			continue
		}
		claim, ok := claims[entry.ClaimID]
		if !ok {
			return project.RepairPacket{}, fmt.Errorf("Inspector failed unknown claim %s", entry.ClaimID)
		}
		packet.FailedClaims = append(packet.FailedClaims, project.RepairClaim{
			ClaimID: entry.ClaimID, Claim: claim.Statement, AttemptedVector: entry.VectorOrAction,
			ObservedEvidence: entry.EvidenceArtifact, FailureReason: strings.TrimSpace(entry.FailureReason),
		})
	}
	if len(packet.FailedClaims) == 0 {
		return project.RepairPacket{}, fmt.Errorf("inspection contains no failed claims")
	}
	packet.PacketDigest = project.RepairPacketDigest(packet)
	return packet, nil
}

func RepairTask(packet project.RepairPacket) string {
	var b strings.Builder
	b.WriteString("Repair only the failed acceptance claims below. Preserve the protected Acceptance Contract; do not edit or regenerate it. Do not change unrelated working behaviour. After the smallest repair, run the declared checks and return the work for fresh independent reinspection.\n\n")
	for _, claim := range packet.FailedClaims {
		fmt.Fprintf(&b, "CLAIM %s: %s\nATTEMPTED VECTOR: %s\nOBSERVED EVIDENCE: %s\nFAILURE: %s\n\n", claim.ClaimID, claim.Claim, claim.AttemptedVector, claim.ObservedEvidence, claim.FailureReason)
	}
	return b.String()
}
