package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const InspectionLedgerSchema = "gary.inspection-ledger.v1"

const (
	InspectionPass               = "PASS"
	InspectionFail               = "FAIL"
	InspectionBlockedEnvironment = "BLOCKED_ENVIRONMENT"
	InspectionBlockedHuman       = "BLOCKED_HUMAN"
	InspectionNotTested          = "NOT_TESTED"
	InspectionNotApplicable      = "NOT_APPLICABLE"
)

// InspectionLedger is the immutable machine-readable result of a fresh
// Inspector run. It records evidence, rather than accepting a worker's prose
// completion claim.
type InspectionLedger struct {
	SchemaVersion      int               `json:"schema_version"`
	ProjectID          string            `json:"project_id"`
	ContractVersion    int               `json:"contract_version"`
	ContractDigest     string            `json:"contract_digest"`
	InspectedCommitSHA string            `json:"inspected_commit_sha"`
	InspectorRunID     string            `json:"inspector_run_id"`
	Timestamp          string            `json:"timestamp"`
	Entries            []InspectionEntry `json:"entries"`
	LedgerDigest       string            `json:"ledger_digest"`
}

type InspectionEntry struct {
	ClaimID          string `json:"claim_id"`
	Claim            string `json:"claim"`
	Boundary         string `json:"boundary"`
	InspectionSeam   string `json:"inspection_seam"`
	VectorOrAction   string `json:"vector_or_action"`
	EvidenceArtifact string `json:"evidence_artifact,omitempty"`
	EvidenceType     string `json:"evidence_type"`
	Result           string `json:"result"`
	FailureReason    string `json:"failure_reason,omitempty"`
	InspectorRunID   string `json:"inspector_run_id"`
	Timestamp        string `json:"timestamp"`
}

func ValidateInspectionLedger(ledger InspectionLedger, contract AcceptanceContract, commitSHA string) error {
	if ledger.SchemaVersion != 1 || ledger.ProjectID != contract.ProjectID || ledger.ContractVersion != contract.ContractVersion || ledger.ContractDigest != contract.ContractDigest || ledger.InspectedCommitSHA != commitSHA || strings.TrimSpace(ledger.InspectorRunID) == "" || strings.TrimSpace(ledger.Timestamp) == "" {
		return errors.New("ledger provenance does not match acceptance contract or inspected commit")
	}
	if len(ledger.Entries) == 0 {
		return errors.New("inspection ledger has no entries")
	}
	claims := make(map[string]AcceptanceClaim, len(contract.Claims))
	for _, claim := range contract.Claims {
		claims[claim.ClaimID] = claim
	}
	seen := map[string]bool{}
	for _, entry := range ledger.Entries {
		claim, ok := claims[entry.ClaimID]
		if !ok || seen[entry.ClaimID] {
			return fmt.Errorf("ledger entry has unknown or repeated claim %q", entry.ClaimID)
		}
		seen[entry.ClaimID] = true
		if strings.TrimSpace(entry.Claim) == "" || entry.Claim != claim.Statement || entry.Boundary != claim.Boundary || strings.TrimSpace(entry.InspectionSeam) == "" || strings.TrimSpace(entry.VectorOrAction) == "" || strings.TrimSpace(entry.EvidenceType) == "" || entry.InspectorRunID != ledger.InspectorRunID || strings.TrimSpace(entry.Timestamp) == "" {
			return fmt.Errorf("ledger entry %s is incomplete or not provenance-bound", entry.ClaimID)
		}
		if !validInspectionResult(entry.Result) {
			return fmt.Errorf("claim %s has invalid result %q", entry.ClaimID, entry.Result)
		}
		if entry.Result == InspectionFail && strings.TrimSpace(entry.FailureReason) == "" {
			return fmt.Errorf("failed claim %s lacks failure reason", entry.ClaimID)
		}
	}
	return nil
}

func validInspectionResult(result string) bool {
	switch result {
	case InspectionPass, InspectionFail, InspectionBlockedEnvironment, InspectionBlockedHuman, InspectionNotTested, InspectionNotApplicable:
		return true
	default:
		return false
	}
}

func InspectionLedgerDigest(ledger InspectionLedger) string {
	ledger.LedgerDigest = ""
	b, _ := json.Marshal(ledger)
	return Digest(b)
}

// SaveInspectionLedger writes a content-addressed immutable ledger and updates
// only the project's latest-evidence pointer. Existing records cannot be
// overwritten with a different result.
func (s Store) SaveInspectionLedger(ledger InspectionLedger) error {
	contract, err := s.LoadAcceptanceContract(ledger.ProjectID)
	if err != nil {
		return err
	}
	if ledger.LedgerDigest == "" {
		ledger.LedgerDigest = InspectionLedgerDigest(ledger)
	}
	if err := ValidateInspectionLedger(ledger, contract, ledger.InspectedCommitSHA); err != nil {
		return err
	}
	path := filepath.Join(s.projectDir(ledger.ProjectID), "evidence", "inspection-"+safeEvidenceID(ledger.InspectorRunID)+".json")
	if _, err := os.ReadFile(path); err == nil {
		var prior InspectionLedger
		if err := readJSON(path, &prior); err != nil || prior.LedgerDigest != ledger.LedgerDigest {
			return errors.New("inspection ledger already exists with different content")
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := writeJSON(path, ledger); err != nil {
		return err
	}
	p, err := s.LoadProject(ledger.ProjectID)
	if err != nil {
		return err
	}
	p.LatestInspectorRunID = ledger.InspectorRunID
	p.LatestEvidenceLedgerDigest = ledger.LedgerDigest
	p.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return writeJSON(filepath.Join(s.projectDir(ledger.ProjectID), "project.json"), p)
}

func (s Store) LoadInspectionLedger(id, runID string) (InspectionLedger, error) {
	if _, err := s.LoadProject(id); err != nil {
		return InspectionLedger{}, err
	}
	var ledger InspectionLedger
	path := filepath.Join(s.projectDir(id), "evidence", "inspection-"+safeEvidenceID(runID)+".json")
	if err := readJSON(path, &ledger); err != nil {
		return InspectionLedger{}, err
	}
	contract, err := s.LoadAcceptanceContract(id)
	if err != nil {
		return InspectionLedger{}, err
	}
	if err := ValidateInspectionLedger(ledger, contract, ledger.InspectedCommitSHA); err != nil {
		return InspectionLedger{}, err
	}
	if ledger.LedgerDigest != InspectionLedgerDigest(ledger) {
		return InspectionLedger{}, errors.New("inspection ledger digest mismatch")
	}
	return ledger, nil
}

func safeEvidenceID(id string) string {
	var b strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "invalid"
	}
	return b.String()
}
