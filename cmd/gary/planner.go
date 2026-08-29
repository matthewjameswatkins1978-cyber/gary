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
	"github.com/matthewjameswatkins1978-cyber/gary/internal/gitops"
)

const (
	yardmasterPlannerTimeout = 5 * time.Minute
	yardmasterContextMaxBytes = 240000
	yardmasterFileMaxBytes = 48000
)

func runYardmasterDirect(cfg *config.Config, manifest *masterManifest, brief, stagingBranch string, wave int, history string) (*masterPlan, time.Duration, error) {
	if cfg.GeminiAPIKey == "" {
		return nil, 0, fmt.Errorf("yardmaster: GEMINI_API_KEY is required on the control plane")
	}

	tmp, err := os.MkdirTemp("", "gary-yardmaster-*")
	if err != nil {
		return nil, 0, err
	}
	defer os.RemoveAll(tmp)

	if err := gitops.CloneBranch(manifest.Repo, stagingBranch, tmp); err != nil {
		return nil, 0, fmt.Errorf("yardmaster clone: %w", err)
	}

	repoContext, err := buildYardmasterRepoContext(tmp, yardmasterContextMaxBytes)
	if err != nil {
		return nil, 0, fmt.Errorf("yardmaster context: %w", err)
	}

	prompt := buildMasterPlannerPrompt(manifest, brief, stagingBranch, wave, history)
	prompt += "\n\nREPOSITORY SNAPSHOT\n===================\n" + repoContext

	ctx, cancel := context.WithTimeout(context.Background(), yardmasterPlannerTimeout)
	defer cancel()

	started := time.Now()
	plan, err := callYardmasterAPI(ctx, cfg, prompt, manifest.MaxJobsPerWave)
	duration := time.Since(started)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, duration, fmt.Errorf("yardmaster exceeded hard planning limit of %s", yardmasterPlannerTimeout)
		}
		return nil, duration, err
	}
	return plan, duration, nil
}

func callYardmasterAPI(ctx context.Context, cfg *config.Config, prompt string, maxJobs int) (*masterPlan, error) {
	endpoint := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", cfg.MasterModel)

	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{{"text": prompt}},
			},
		},
		"generationConfig": map[string]interface{}{
			"thinkingConfig": map[string]interface{}{
				"thinkingLevel": "medium",
			},
			"responseMimeType": "application/json",
			"maxOutputTokens":  4096,
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("yardmaster request encode: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("yardmaster request build: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", cfg.GeminiAPIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("yardmaster transport: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("yardmaster response read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("yardmaster API %d: %s", resp.StatusCode, compactPlannerError(string(respBody), 2000))
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
		return nil, fmt.Errorf("yardmaster response parse: %w", err)
	}
	if len(apiResp.Candidates) == 0 {
		return nil, fmt.Errorf("yardmaster returned no candidates")
	}

	var raw strings.Builder
	for _, part := range apiResp.Candidates[0].Content.Parts {
		raw.WriteString(part.Text)
	}
	text := strings.TrimSpace(raw.String())
	text = stripJSONFence(text)

	var plan masterPlan
	dec := json.NewDecoder(strings.NewReader(text))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&plan); err != nil {
		return nil, fmt.Errorf("yardmaster plan JSON: %w; output=%s", err, compactPlannerError(text, 2000))
	}
	if err := validateMasterPlan(&plan, maxJobs); err != nil {
		return nil, err
	}
	return &plan, nil
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
	files := strings.Fields(fileList)
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
