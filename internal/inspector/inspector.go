// Package inspector contains the role boundary and wire validation for fresh
// independent acceptance inspections. It deliberately has no worker state or
// retry-history input in its public prompt builder.
package inspector

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/project"
)

type PromptInput struct {
	Context               project.ProjectContext
	InspectedCommitSHA    string
	InspectorRunID        string
	RepositorySnapshot    string
	BuilderClaims         []string
	PreviousLedger        string
	RealEvidenceAvailable bool
}

// BuildPrompt constructs a fresh Inspector context from durable project data,
// repository evidence and completion claims only. Builder reasoning, repair
// prose and conversational history are intentionally not parameters.
func BuildPrompt(input PromptInput) string {
	b, _ := json.MarshalIndent(struct {
		Project      project.Project              `json:"project"`
		Manual       project.ProjectManual        `json:"manual"`
		Contract     project.AcceptanceContract   `json:"acceptance_contract"`
		Architecture project.ArchitectureManifest `json:"architecture_manifest,omitempty"`
	}{input.Context.Project, input.Context.Manual, input.Context.Contract, input.Context.Architecture}, "", "  ")
	claims := strings.Join(input.BuilderClaims, "\n- ")
	if claims == "" {
		claims = "none supplied"
	}
	previous := input.PreviousLedger
	if strings.TrimSpace(previous) == "" {
		previous = "none; this is the initial inspection"
	}
	return fmt.Sprintf(`You are the GARY Independent Inspector. This is a fresh context, separate from the Builder.

Your only objective is to classify whether each material Acceptance Contract claim is supported by evidence at the inspected commit. Do not accept a Builder statement as proof. Do not redesign the application or write a replacement test harness. Inspect source/build state, use declared seams and ordinary build/test tools, and invent small adversarial vectors where useful.

Return exactly one JSON object with an "entries" array. Emit one entry for every required claim. Each entry must contain: claim_id, claim, boundary, inspection_seam, vector_or_action, evidence_type, result, failure_reason, inspector_run_id and timestamp. Result must be one of PASS, FAIL, BLOCKED_ENVIRONMENT, BLOCKED_HUMAN, NOT_TESTED or NOT_APPLICABLE. Evidence type must be one of static, unit, synthetic, component_real, integration_real, environment_real or human_real. A real-world claim cannot be PASS on synthetic evidence when direct evidence is available. Hardware unavailable is BLOCKED_ENVIRONMENT, not PASS and not an automatic global stop.

DURABLE INSPECTION INPUT
========================
%s

INSPECTED COMMIT: %s
INSPECTOR RUN ID: %s
DIRECT REAL EVIDENCE AVAILABLE: %t

REPOSITORY SNAPSHOT
===================
%s

BUILDER COMPLETION CLAIMS (claims only; no Builder reasoning)
=============================================================
- %s

PREVIOUS FALSIFICATION LEDGER (use only for targeted reinspection context)
=========================================================================
%s`, string(b), input.InspectedCommitSHA, input.InspectorRunID, input.RealEvidenceAvailable, input.RepositorySnapshot, claims, previous)
}

// DecodeResponse accepts only the small machine-readable response surface the
// Inspector is allowed to emit. Orchestration fills immutable provenance from
// the call, never from model-controlled project identity fields.
func DecodeResponse(text string, ctx project.ProjectContext, commitSHA, runID string, requireAll bool) (project.InspectionLedger, error) {
	text = stripJSONFence(strings.TrimSpace(text))
	var response struct {
		Entries []project.InspectionEntry `json:"entries"`
	}
	dec := json.NewDecoder(strings.NewReader(text))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&response); err != nil {
		return project.InspectionLedger{}, fmt.Errorf("decode inspector response: %w", err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return project.InspectionLedger{}, errors.New("inspector response contains more than one JSON value")
	}
	if len(response.Entries) == 0 {
		return project.InspectionLedger{}, errors.New("inspector returned no ledger entries")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for i := range response.Entries {
		if response.Entries[i].InspectorRunID == "" {
			response.Entries[i].InspectorRunID = runID
		}
		if response.Entries[i].Timestamp == "" {
			response.Entries[i].Timestamp = now
		}
		if response.Entries[i].EvidenceType == "" {
			return project.InspectionLedger{}, fmt.Errorf("entry %d lacks evidence_type", i)
		}
		if !validEvidenceType(response.Entries[i].EvidenceType) {
			return project.InspectionLedger{}, fmt.Errorf("entry %d has invalid evidence_type %q", i, response.Entries[i].EvidenceType)
		}
	}
	ledger := project.InspectionLedger{
		SchemaVersion: 1, ProjectID: ctx.Project.ProjectID, ContractVersion: ctx.Contract.ContractVersion,
		ContractDigest: ctx.Contract.ContractDigest, InspectedCommitSHA: commitSHA,
		InspectorRunID: runID, Timestamp: now, Entries: response.Entries,
	}
	for i := range ledger.Entries {
		ledger.Entries[i].InspectorRunID = runID
	}
	if requireAll {
		for _, claim := range ctx.Contract.Claims {
			found := false
			for _, entry := range ledger.Entries {
				if entry.ClaimID == claim.ClaimID {
					found = true
					break
				}
			}
			if claim.Importance == "required" && !found {
				return project.InspectionLedger{}, fmt.Errorf("required claim %s missing from ledger", claim.ClaimID)
			}
		}
	}
	if err := project.ValidateInspectionLedger(ledger, ctx.Contract, commitSHA); err != nil {
		return project.InspectionLedger{}, err
	}
	ledger.LedgerDigest = project.InspectionLedgerDigest(ledger)
	return ledger, nil
}

func stripJSONFence(text string) string {
	if strings.HasPrefix(text, "```") {
		if newline := strings.IndexByte(text, '\n'); newline >= 0 {
			text = text[newline+1:]
		}
		text = strings.TrimSuffix(text, "```")
	}
	return strings.TrimSpace(text)
}

func validEvidenceType(value string) bool {
	switch value {
	case "static", "unit", "synthetic", "component_real", "integration_real", "environment_real", "human_real":
		return true
	default:
		return false
	}
}

// EvidenceParity reports whether an evidence class can close a claim's
// boundary. It is intentionally coarse; the Inspector still supplies the
// claim-level judgement and ledger entry.
func EvidenceParity(claim project.AcceptanceClaim, evidenceType string, directEvidenceAvailable bool) bool {
	if claim.Boundary == "physical_device" {
		return evidenceType == "environment_real" || evidenceType == "human_real"
	}
	if claim.Boundary == "real_external_service" && directEvidenceAvailable {
		return evidenceType == "integration_real" || evidenceType == "environment_real" || evidenceType == "human_real"
	}
	if claim.Boundary == "process_and_persistent_store" && directEvidenceAvailable {
		return evidenceType == "component_real" || evidenceType == "integration_real" || evidenceType == "environment_real" || evidenceType == "human_real"
	}
	return true
}
