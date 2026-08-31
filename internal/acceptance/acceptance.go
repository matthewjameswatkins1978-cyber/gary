// Package acceptance translates Inspector evidence into narrow repair work.
package acceptance

import (
	"fmt"
	"strings"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/inspector"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/project"
)

type Evaluation struct {
	Status   string   `json:"status"`
	Failures []string `json:"failures,omitempty"`
	Blocked  []string `json:"blocked,omitempty"`
}

// Evaluate applies the cheap, deterministic part of acceptance after an
// Inspector response. It prevents evidence substitution even if a model
// mistakenly labels a synthetic result PASS.
func Evaluate(contract project.AcceptanceContract, ledger project.InspectionLedger, directEvidenceAvailable bool) Evaluation {
	entries := map[string]project.InspectionEntry{}
	for _, entry := range ledger.Entries {
		entries[entry.ClaimID] = entry
	}
	result := Evaluation{Status: "PASS"}
	for _, claim := range contract.Claims {
		if claim.Importance != "required" {
			continue
		}
		entry, ok := entries[claim.ClaimID]
		if !ok {
			result.Failures = append(result.Failures, claim.ClaimID+": missing Inspector result")
			continue
		}
		switch entry.Result {
		case project.InspectionFail:
			result.Failures = append(result.Failures, claim.ClaimID+": "+entry.FailureReason)
		case project.InspectionBlockedEnvironment, project.InspectionBlockedHuman, project.InspectionNotTested:
			result.Blocked = append(result.Blocked, claim.ClaimID+": "+entry.Result)
		case project.InspectionPass:
			if !inspector.EvidenceParity(claim, entry.EvidenceType, directEvidenceAvailable) {
				result.Failures = append(result.Failures, claim.ClaimID+": evidence parity failure")
			}
		}
	}
	if len(result.Failures) > 0 {
		result.Status = "FAIL"
	} else if len(result.Blocked) > 0 {
		result.Status = "ACCEPTED_WITH_BLOCKED_VERIFICATION"
	}
	return result
}

func BuildRepairPacket(contract project.AcceptanceContract, ledger project.InspectionLedger) (project.RepairPacket, error) {
	return buildRepairPacket(contract, ledger, nil)
}

// BuildRepairPacketForEvaluation also routes deterministic evidence-parity
// failures, which may not have been emitted as a literal Inspector FAIL row,
// into a narrow repair packet.
func BuildRepairPacketForEvaluation(contract project.AcceptanceContract, ledger project.InspectionLedger, evaluation Evaluation) (project.RepairPacket, error) {
	return buildRepairPacket(contract, ledger, &evaluation)
}

func buildRepairPacket(contract project.AcceptanceContract, ledger project.InspectionLedger, evaluation *Evaluation) (project.RepairPacket, error) {
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
	if evaluation != nil {
		seen := map[string]bool{}
		for _, claim := range packet.FailedClaims {
			seen[claim.ClaimID] = true
		}
		for _, failure := range evaluation.Failures {
			claimID := strings.TrimSpace(strings.SplitN(failure, ":", 2)[0])
			claim, ok := claims[claimID]
			if !ok || seen[claimID] {
				continue
			}
			packet.FailedClaims = append(packet.FailedClaims, project.RepairClaim{
				ClaimID: claimID, Claim: claim.Statement, FailureReason: failure,
			})
		}
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
