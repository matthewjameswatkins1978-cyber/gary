package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/architecture"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/config"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/cost"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/gitops"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/inspector"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/project"
)

const inspectorTimeout = 5 * time.Minute

// runMasterInspector is a fresh control-plane model call. It receives a
// separately constructed Inspector prompt, never the Builder's conversation,
// retry narrative or private reasoning.
func runMasterInspector(ctx context.Context, cfg *config.Config, input project.ProjectContext, manifest *masterManifest, stagingBranch string, builderClaims []string) (project.InspectionLedger, cost.Component, error) {
	component := estimatedComponent("inspector", cfg.InspectorModel, "inspector request did not return provider usage")
	if input.Architecture.ManifestVersion > 0 {
		check := architecture.Validate(input.Architecture, input.Contract)
		if len(check.Errors) > 0 {
			return project.InspectionLedger{}, component, fmt.Errorf("architecture preflight failed: %s", strings.Join(check.Errors, "; "))
		}
	}
	tmp, err := os.MkdirTemp("", "gary-inspector-*")
	if err != nil {
		return project.InspectionLedger{}, component, err
	}
	defer os.RemoveAll(tmp)
	if err := gitops.CloneBranch(manifest.Repo, stagingBranch, tmp); err != nil {
		return project.InspectionLedger{}, component, fmt.Errorf("inspector clone: %w", err)
	}
	commitSHA, err := gitops.FullCommitSHA(tmp)
	if err != nil {
		return project.InspectionLedger{}, component, fmt.Errorf("inspector commit identity: %w", err)
	}
	repoContext, err := buildYardmasterRepoContext(tmp, yardmasterContextMaxBytes)
	if err != nil {
		return project.InspectionLedger{}, component, fmt.Errorf("inspector repository snapshot: %w", err)
	}
	runID := fmt.Sprintf("inspector-%s-%d", input.Project.ProjectID, time.Now().UTC().UnixNano())
	prompt := inspector.BuildPrompt(inspector.PromptInput{
		Context: input, InspectedCommitSHA: commitSHA, InspectorRunID: runID,
		RepositorySnapshot: repoContext, BuilderClaims: builderClaims,
		RealEvidenceAvailable: true,
	})
	response, component, err := callInspectorAPI(ctx, cfg, prompt)
	if err != nil {
		return project.InspectionLedger{}, component, err
	}
	ledger, err := inspector.DecodeResponse(response, input, commitSHA, runID, true)
	if err != nil {
		return project.InspectionLedger{}, component, err
	}
	if err := project.DefaultStore().SaveInspectionLedger(ledger); err != nil {
		return project.InspectionLedger{}, component, fmt.Errorf("persist inspector ledger: %w", err)
	}
	return ledger, component, nil
}

func callInspectorAPI(ctx context.Context, cfg *config.Config, prompt string) (string, cost.Component, error) {
	component := estimatedComponent("inspector", cfg.InspectorModel, "inspector request did not return provider usage")
	token, err := vertexAccessToken(ctx)
	if err != nil {
		return "", component, fmt.Errorf("inspector Vertex auth: %w", err)
	}
	body, err := json.Marshal(map[string]interface{}{
		"contents": []map[string]interface{}{{"role": "user", "parts": []map[string]string{{"text": prompt}}}},
		"generationConfig": map[string]interface{}{
			"thinkingConfig":   map[string]interface{}{"thinkingLevel": "LOW"},
			"responseMimeType": "application/json", "maxOutputTokens": 4096,
		},
	})
	if err != nil {
		return "", component, fmt.Errorf("inspector request encode: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, yardmasterVertexEndpoint(cfg.GCPProject, cfg.InspectorModel), bytes.NewReader(body))
	if err != nil {
		return "", component, fmt.Errorf("inspector request build: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", component, fmt.Errorf("inspector transport: %w", err)
	}
	defer resp.Body.Close()
	component.ProviderRequestID = resp.Header.Get("x-goog-request-id")
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", component, fmt.Errorf("inspector response read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		component.UnmeteredReason = fmt.Sprintf("inspector HTTP %d returned no usable usage record", resp.StatusCode)
		return "", component, fmt.Errorf("inspector API %d: %s", resp.StatusCode, compactPlannerError(string(data), 2000))
	}
	if usage, usageErr := cost.ParseVertexUsage(json.RawMessage(data), cfg.InspectorModel, "", component.ProviderRequestID); usageErr == nil {
		component = componentFromUsage("inspector", usage)
		component.ProviderRequestID = resp.Header.Get("x-goog-request-id")
	}
	var apiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(data, &apiResp); err != nil {
		return "", component, fmt.Errorf("inspector response parse: %w", err)
	}
	if len(apiResp.Candidates) == 0 {
		return "", component, fmt.Errorf("inspector returned no candidates")
	}
	var text strings.Builder
	for _, part := range apiResp.Candidates[0].Content.Parts {
		text.WriteString(part.Text)
	}
	return stripJSONFence(strings.TrimSpace(text.String())), component, nil
}

func inspectorHasBlockingFailure(ledger project.InspectionLedger) (bool, string) {
	for _, entry := range ledger.Entries {
		if entry.Result == project.InspectionFail {
			return true, fmt.Sprintf("Inspector failed claim %s: %s", entry.ClaimID, entry.FailureReason)
		}
	}
	return false, ""
}

func inspectorHasExternalBlock(ledger project.InspectionLedger) bool {
	for _, entry := range ledger.Entries {
		if entry.Result == project.InspectionBlockedEnvironment || entry.Result == project.InspectionBlockedHuman || entry.Result == project.InspectionNotTested {
			return true
		}
	}
	return false
}

func inspectorLedgerPath(projectID, runID string) string {
	return filepath.Join(project.DefaultStore().Root(), projectID, "evidence", "inspection-"+runID+".json")
}
