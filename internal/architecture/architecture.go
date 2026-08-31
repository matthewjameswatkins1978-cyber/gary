// Package architecture validates the small machine-readable architecture
// manifest used by substantial projects. It reports warnings separately so
// legitimate engineering necessities are not confused with hard failures.
package architecture

import (
	"fmt"
	"strings"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/project"
)

type CheckResult struct {
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

func Validate(manifest project.ArchitectureManifest, contract project.AcceptanceContract) CheckResult {
	result := CheckResult{}
	if err := project.ValidateArchitectureManifest(manifest, contract.ProjectID, contract.ContractVersion, contract.ContractDigest, manifest.ManifestVersion); err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result
	}
	claims := map[string]bool{}
	for _, claim := range contract.Claims {
		if claim.Importance == "required" {
			claims[claim.ClaimID] = true
		}
	}
	supported := map[string]bool{}
	for _, component := range manifest.Components {
		for _, claimID := range component.Supports {
			if !claims[claimID] {
				result.Warnings = append(result.Warnings, fmt.Sprintf("component %s references non-live claim %s", component.ID, claimID))
			} else {
				supported[claimID] = true
			}
		}
		lower := strings.ToLower(component.ID)
		if isExternalInfrastructure(lower) && component.JustificationType != "engineering_necessity" && component.JustificationType != "explicit_requirement" {
			result.Warnings = append(result.Warnings, fmt.Sprintf("component %s introduces external infrastructure without a direct requirement", component.ID))
		}
	}
	for claimID := range claims {
		if !supported[claimID] {
			result.Warnings = append(result.Warnings, fmt.Sprintf("required claim %s has no architecture lineage", claimID))
		}
	}
	return result
}

func Compare(previous, current project.ArchitectureManifest) CheckResult {
	result := CheckResult{}
	prior := map[string]bool{}
	for _, component := range previous.Components {
		prior[component.ID] = true
	}
	for _, component := range current.Components {
		if !prior[component.ID] {
			result.Warnings = append(result.Warnings, fmt.Sprintf("repair added architecture component %s", component.ID))
		}
	}
	return result
}

func isExternalInfrastructure(id string) bool {
	for _, marker := range []string{"redis", "postgres", "kafka", "queue", "database", "service", "cluster"} {
		if strings.Contains(id, marker) {
			return true
		}
	}
	return false
}
