package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/config"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/cost"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/gitops"
)

const (
	yardmasterPlannerTimeout  = 5 * time.Minute
	yardmasterContextMaxBytes = 240000
	yardmasterFileMaxBytes    = 48000
)

func runYardmasterDirect(cfg *config.Config, manifest *masterManifest, brief, stagingBranch string, wave int, history, memory string) (*masterPlan, time.Duration, cost.Component, error) {
	component := estimatedComponent("yardmaster", cfg.MasterModel, "planner request did not return provider usage")
	if cfg.GCPProject == "" {
		return nil, 0, component, fmt.Errorf("yardmaster: GARY_GCP_PROJECT is required on the control plane")
	}

	tmp, err := os.MkdirTemp("", "gary-yardmaster-*")
	if err != nil {
		return nil, 0, component, err
	}
	defer os.RemoveAll(tmp)

	if err := gitops.CloneBranch(manifest.Repo, stagingBranch, tmp); err != nil {
		return nil, 0, component, fmt.Errorf("yardmaster clone: %w", err)
	}

	repoContext, err := buildYardmasterRepoContext(tmp, yardmasterContextMaxBytes)
	if err != nil {
		return nil, 0, component, fmt.Errorf("yardmaster context: %w", err)
	}

	prompt := buildMasterPlannerPrompt(manifest, brief, stagingBranch, wave, history, memory)
	prompt += "\n\nREPOSITORY SNAPSHOT\n===================\n" + repoContext

	ctx, cancel := context.WithTimeout(context.Background(), yardmasterPlannerTimeout)
	defer cancel()

	started := time.Now()
	plan, component, err := callYardmasterAPI(ctx, cfg, prompt, manifest.MaxJobsPerWave)
	duration := time.Since(started)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, duration, component, fmt.Errorf("yardmaster exceeded hard planning limit of %s", yardmasterPlannerTimeout)
		}
		return nil, duration, component, err
	}
	return plan, duration, component, nil
}

func callYardmasterAPI(ctx context.Context, cfg *config.Config, prompt string, maxJobs int) (*masterPlan, cost.Component, error) {
	component := estimatedComponent("yardmaster", cfg.MasterModel, "planner request did not return provider usage")
	token, err := vertexAccessToken(ctx)
	if err != nil {
		return nil, component, fmt.Errorf("yardmaster Vertex auth: %w", err)
	}
	endpoint := yardmasterVertexEndpoint(cfg.GCPProject, cfg.MasterModel)
	payload, err := json.Marshal(yardmasterRequestBody(prompt))
	if err != nil {
		return nil, component, fmt.Errorf("yardmaster request encode: %w", err)
	}

	const maxAttempts = 3
	var respBody []byte
	var providerRequestID string
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			return nil, component, fmt.Errorf("yardmaster request build: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, component, fmt.Errorf("yardmaster transport: %w", err)
		}
		providerRequestID = resp.Header.Get("x-goog-request-id")
		component.ProviderRequestID = providerRequestID
		respBody, err = io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if err != nil {
			return nil, component, fmt.Errorf("yardmaster response read: %w", err)
		}
		if resp.StatusCode == http.StatusTooManyRequests && attempt < maxAttempts {
			delay := time.Duration(attempt*5) * time.Second
			fmt.Fprintf(os.Stderr, "[yardmaster] API 429; retrying attempt %d/%d after %s\n", attempt+1, maxAttempts, delay)
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, component, ctx.Err()
			case <-timer.C:
			}
			continue
		}
		if resp.StatusCode != http.StatusOK {
			component.UnmeteredReason = fmt.Sprintf("yardmaster HTTP %d returned no usable usage record", resp.StatusCode)
			return nil, component, fmt.Errorf("yardmaster API %d: %s", resp.StatusCode, compactPlannerError(string(respBody), 2000))
		}
		break
	}

	if usage, usageErr := cost.ParseVertexUsage(json.RawMessage(respBody), cfg.MasterModel, "", providerRequestID); usageErr == nil {
		component = componentFromUsage("yardmaster", usage)
		component.ProviderRequestID = providerRequestID
	} else {
		component.UnmeteredReason = "yardmaster response lacked complete usageMetadata: " + usageErr.Error()
	}

	var apiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, component, fmt.Errorf("yardmaster response parse: %w", err)
	}
	if len(apiResp.Candidates) == 0 {
		return nil, component, fmt.Errorf("yardmaster returned no candidates")
	}

	var raw strings.Builder
	for _, part := range apiResp.Candidates[0].Content.Parts {
		raw.WriteString(part.Text)
	}
	text := stripJSONFence(strings.TrimSpace(raw.String()))

	var plan masterPlan
	dec := json.NewDecoder(strings.NewReader(text))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&plan); err != nil {
		return nil, component, fmt.Errorf("yardmaster plan JSON: %w; output=%s", err, compactPlannerError(text, 2000))
	}
	if err := validateMasterPlan(&plan, maxJobs); err != nil {
		return nil, component, err
	}
	return enforceEconomicPlan(&plan), component, nil
}

func estimatedComponent(kind, model, reason string) cost.Component {
	return cost.Component{
		Kind:                 kind,
		ModelRequested:       model,
		UsageSource:          cost.Estimated,
		PricingVersion:       cost.PricingVersion,
		PricingEffectiveDate: cost.PricingEffectiveDate,
		PricingSource:        cost.PricingSource,
		UnmeteredReason:      reason,
	}
}

func componentFromUsage(kind string, usage cost.Usage) cost.Component {
	pt := cost.PriceForModel(usage.ModelRequested)
	c := cost.Component{
		Kind:                 kind,
		ModelRequested:       usage.ModelRequested,
		ModelVersion:         usage.ModelVersion,
		UsageSource:          usage.UsageSource,
		InputTokens:          usage.InputTokens,
		CachedInputTokens:    usage.CachedInputTokens,
		FreshInputTokens:     usage.FreshInputTokens,
		OutputTokens:         usage.OutputTokens,
		ThinkingTokens:       usage.ThinkingTokens,
		TotalTokens:          usage.TotalTokens,
		PricingVersion:       pt.Version,
		PricingEffectiveDate: pt.EffectiveDate,
		PricingSource:        pt.Source,
		ProviderResponseID:   usage.ProviderResponseID,
		ProviderRequestID:    usage.ProviderRequestID,
	}
	if !pt.Known {
		c.UsageSource = cost.Estimated
		c.UnmeteredReason = "no versioned price for requested model " + usage.ModelRequested
		return c
	}
	c.ModelCostUSD = cost.ModelCost(pt, usage.FreshInputTokens, usage.CachedInputTokens, usage.OutputTokens)
	c.TotalCostUSD = c.ModelCostUSD
	return c
}

func yardmasterRequestBody(prompt string) map[string]interface{} {
	return map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role":  "user",
				"parts": []map[string]string{{"text": prompt}},
			},
		},
		"generationConfig": map[string]interface{}{
			"thinkingConfig": map[string]interface{}{
				"thinkingLevel": "MEDIUM",
			},
			"responseMimeType": "application/json",
			"maxOutputTokens":  8192,
		},
	}
}

func yardmasterVertexEndpoint(project, model string) string {
	return fmt.Sprintf(
		"https://aiplatform.googleapis.com/v1/projects/%s/locations/global/publishers/google/models/%s:generateContent",
		project,
		model,
	)
}

func vertexAccessToken(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "gcloud", "auth", "print-access-token")
	cmd.WaitDelay = 2 * time.Second
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gcloud auth print-access-token: %w: %s", err, strings.TrimSpace(string(out)))
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", fmt.Errorf("gcloud auth print-access-token returned an empty token")
	}
	return token, nil
}

func buildYardmasterRepoContext(repoDir string, maxBytes int) (string, error) {
	if maxBytes <= 0 {
		return "", nil
	}

	var out strings.Builder
	writeBudgeted := func(s string) bool {
		if s == "" {
			return true
		}
		remaining := maxBytes - out.Len()
		if remaining <= 0 {
			return false
		}
		if len(s) > remaining {
			out.WriteString(s[:remaining])
			return false
		}
		out.WriteString(s)
		return true
	}

	if head, err := plannerCommandOutput(repoDir, "git", "log", "-1", "--oneline"); err == nil {
		writeBudgeted("HEAD: " + head + "\n\n")
	}

	fileList, err := plannerCommandOutput(repoDir, "git", "ls-files")
	if err != nil {
		return "", err
	}
	files := make([]string, 0)
	for _, name := range strings.Fields(fileList) {
		if !plannerDisposablePath(name) {
			files = append(files, name)
		}
	}
	writeBudgeted("TRACKED FILES:\n")
	for _, name := range files {
		if !writeBudgeted(name + "\n") {
			break
		}
	}
	writeBudgeted("\n")

	sort.SliceStable(files, func(i, j int) bool {
		pi, pj := plannerFilePriority(files[i]), plannerFilePriority(files[j])
		if pi != pj {
			return pi < pj
		}
		return files[i] < files[j]
	})

	for _, rel := range files {
		if out.Len() >= maxBytes {
			break
		}
		if !plannerTextFile(rel) {
			continue
		}
		full := filepath.Join(repoDir, filepath.FromSlash(rel))
		info, err := os.Stat(full)
		if err != nil || info.IsDir() || info.Size() == 0 {
			continue
		}
		data, err := os.ReadFile(full)
		if err != nil || bytes.IndexByte(data, 0) >= 0 {
			continue
		}
		if len(data) > yardmasterFileMaxBytes {
			data = data[:yardmasterFileMaxBytes]
		}
		header := fmt.Sprintf("\n--- FILE: %s ---\n", rel)
		if !writeBudgeted(header) || !writeBudgeted(string(data)) || !writeBudgeted("\n") {
			break
		}
	}

	if out.Len() >= maxBytes {
		out.WriteString("\n[repository snapshot truncated]\n")
	}
	return out.String(), nil
}

func plannerFilePriority(path string) int {
	lower := strings.ToLower(filepath.ToSlash(path))
	base := strings.ToLower(filepath.Base(lower))
	switch base {
	case "agents.md", "readme.md", "gary_rulebook.md", "go.mod", "package.json", "pyproject.toml", "cargo.toml", "requirements.txt":
		return 0
	}
	for _, prefix := range []string{"cmd/", "internal/", "src/", "app/", "server/", "control/"} {
		if strings.HasPrefix(lower, prefix) {
			return 1
		}
	}
	if strings.HasPrefix(lower, "docs/") {
		return 3
	}
	return 2
}

func plannerTextFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".rs", ".py", ".ts", ".tsx", ".js", ".jsx", ".json", ".md", ".toml", ".yaml", ".yml", ".sh", ".ps1", ".txt":
		return true
	default:
		return false
	}
}

func plannerDisposablePath(path string) bool {
	parts := strings.Split(strings.ToLower(filepath.ToSlash(path)), "/")
	for _, part := range parts {
		switch part {
		case ".venv", "__pycache__", ".pytest_cache", ".mypy_cache", ".ruff_cache", ".tox", ".nox", ".swiftpm", ".build", "deriveddata":
			return true
		}
		if strings.HasSuffix(part, ".egg-info") {
			return true
		}
	}
	return false
}

func plannerCommandOutput(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func stripJSONFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimSuffix(strings.TrimSpace(s), "```")
		return strings.TrimSpace(s)
	}
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(strings.TrimSpace(s), "```")
		return strings.TrimSpace(s)
	}
	return s
}

func compactPlannerError(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return "[truncated] " + s[len(s)-max:]
}
