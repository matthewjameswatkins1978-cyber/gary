package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/batch"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/config"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/cost"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/githubapp"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/gitops"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/jobs"
)

const cloudCommandTimeout = 45 * time.Second

func currentBatchGitHubToken(repoURL string) (string, error) {
	return githubapp.TokenFromEnvForRepository(repoURL)
}

func randomSuffix(n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)[:n]
}

func downloadGCSFile(tmpDir, gcsPath string) string {
	path := filepath.Join(tmpDir, filepath.Base(gcsPath))
	commands := [][]string{
		{"gcloud", "storage", "cp", gcsPath, path},
		{"python3", "-m", "gsutil", "cp", gcsPath, path},
	}
	for _, argv := range commands {
		ctx, cancel := context.WithTimeout(context.Background(), cloudCommandTimeout)
		cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
		cmd.WaitDelay = 2 * time.Second
		_, err := cmd.CombinedOutput()
		cancel()
		if err != nil {
			continue
		}
		data, err := os.ReadFile(path)
		if err == nil {
			return string(data)
		}
	}
	return ""
}

type batchLogEntry struct {
	Labels   map[string]string `json:"labels"`
	Resource struct {
		Labels map[string]string `json:"labels"`
	} `json:"resource"`
	TextPayload string `json:"textPayload"`
	Timestamp   string `json:"timestamp"`
}

func extractFromBatchLogs(project, region, jobName, marker string) string {
	status, err := batch.DescribeJob(project, region, jobName)
	if err != nil || status.UID == "" || status.CreateTime == "" || status.UpdateTime == "" {
		return ""
	}
	filter := fmt.Sprintf(
		`logName="projects/%s/logs/batch_task_logs" AND timestamp >= "%s" AND timestamp <= "%s"`,
		project, status.CreateTime, status.UpdateTime,
	)
	ctx, cancel := context.WithTimeout(context.Background(), cloudCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gcloud", "logging", "read", filter,
		"--project", project,
		"--format", "json",
		"--limit", "10000",
	)
	cmd.WaitDelay = 2 * time.Second
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	var entries []batchLogEntry
	if err := json.Unmarshal(out, &entries); err != nil {
		return ""
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Timestamp < entries[j].Timestamp })
	return extractMarkerFromBatchLogs(entries, status.UID, marker)
}

func extractMarkerFromBatchLogs(entries []batchLogEntry, jobUID, marker string) string {
	startMarker := marker + "_START"
	endMarker := marker + "_END"
	var lines []string
	capturing := false
	for _, entry := range entries {
		if entry.Labels["job_uid"] != jobUID && entry.Resource.Labels["job_id"] != jobUID {
			continue
		}
		line := strings.TrimSpace(entry.TextPayload)
		if strings.Contains(line, startMarker) {
			capturing = true
			continue
		}
		if strings.Contains(line, endMarker) {
			break
		}
		if capturing && line != "" && !strings.HasPrefix(line, "GARY_") {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.Join(lines, ""))
	if err != nil {
		return ""
	}
	return string(decoded)
}

type localAcceptanceResult struct {
	passed  bool
	blocker bool
	err     error
}

func runLocalAcceptanceClassified(tmpDir string, job *jobs.Job, garyBranch, patchData, prefix string) localAcceptanceResult {
	fail := func(err error, blocker bool) localAcceptanceResult {
		return localAcceptanceResult{blocker: blocker, err: err}
	}
	workDir := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return fail(err, true)
	}
	if err := gitops.Clone(job.Repo, workDir); err != nil {
		return fail(err, isServiceEnvironmentBlocker(err.Error()))
	}
	if err := gitops.Checkout(workDir, job.BaseBranch); err != nil {
		return fail(err, isServiceEnvironmentBlocker(err.Error()))
	}
	if err := gitops.CreateBranch(workDir, garyBranch); err != nil {
		return fail(err, isServiceEnvironmentBlocker(err.Error()))
	}
	if err := gitops.ApplyPatch(workDir, patchData); err != nil {
		return fail(err, false)
	}

	pythonBin, cleanupPython, err := preparePythonAcceptance(workDir, job.Acceptance)
	if err != nil {
		return fail(err, true)
	}
	defer cleanupPython()

	for _, command := range job.Acceptance {
		out, err := runAcceptanceCommand(workDir, command, pythonBin)
		if err != nil {
			return fail(fmt.Errorf("local acceptance command %q failed: %w\n%s", command, err, strings.TrimSpace(string(out))), false)
		}
	}
	if err := cleanupDisposableWorkerArtifacts(workDir); err != nil {
		return fail(err, true)
	}
	if err := gitops.CommitAll(workDir, fmt.Sprintf("GARY: %s", job.ID)); err != nil {
		return fail(err, isServiceEnvironmentBlocker(err.Error()))
	}
	return localAcceptanceResult{passed: true}
}

func cleanupDisposableWorkerArtifacts(root string) error {
	disposable := map[string]bool{
		".venv":         true,
		"__pycache__":   true,
		".pytest_cache": true,
		".mypy_cache":   true,
		".ruff_cache":   true,
		".tox":          true,
		".nox":          true,
		".swiftpm":      true,
		".build":        true,
		"target":        true,
		"DerivedData":   true,
	}
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root || !entry.IsDir() {
			return nil
		}
		if disposable[entry.Name()] || strings.HasSuffix(entry.Name(), ".egg-info") {
			if err := os.RemoveAll(path); err != nil {
				return err
			}
			return filepath.SkipDir
		}
		return nil
	})
}

func isServiceEnvironmentBlocker(message string) bool {
	lower := strings.ToLower(message)
	for _, marker := range []string{
		"github authentication",
		"could not read username for 'https://github.com'",
		"authentication failed for 'https://github.com",
		"remote: invalid username or password",
		"http 401",
		"http 403",
		"gcloud authentication",
		"could not load the default credentials",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func preparePythonAcceptance(workDir string, commands []string) (string, func(), error) {
	if !usesPythonAcceptance(commands) {
		return "", func() {}, nil
	}
	python := "python3"
	if _, err := exec.LookPath(python); err != nil {
		python = "python"
	}
	if _, err := exec.LookPath(python); err != nil {
		return "", func() {}, errors.New("Python acceptance requested but no Python interpreter is available")
	}
	venvDir := filepath.Join(workDir, ".venv")
	if out, err := exec.Command(python, "-m", "venv", venvDir).CombinedOutput(); err != nil {
		return "", func() {}, fmt.Errorf("create Python virtualenv: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	pythonBin := filepath.Join(venvDir, "bin", "python")
	if _, err := os.Stat(pythonBin); err != nil {
		pythonBin = filepath.Join(venvDir, "Scripts", "python.exe")
	}
	if _, err := os.Stat(pythonBin); err != nil {
		return "", func() {}, errors.New("Python virtualenv did not contain an interpreter")
	}
	if !hasPythonInstall(commands) {
		args := []string{"-m", "pip", "install"}
		if _, err := os.Stat(filepath.Join(workDir, "pyproject.toml")); err == nil {
			args = append(args, "-e", ".[dev,deploy]")
		} else if _, err := os.Stat(filepath.Join(workDir, "setup.py")); err == nil {
			args = append(args, "-e", ".[dev,deploy]")
		} else {
			args = append(args, "pytest", "ruff", "build")
		}
		cmd := exec.Command(pythonBin, args...)
		cmd.Dir = workDir
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", func() {}, fmt.Errorf("install Python development environment: %w\n%s", err, strings.TrimSpace(string(out)))
		}
	}
	return pythonBin, func() { _ = os.RemoveAll(venvDir) }, nil
}

func usesPythonAcceptance(commands []string) bool {
	for _, command := range commands {
		for _, segment := range acceptanceCommandSegments(command) {
			parts := strings.Fields(segment)
			if len(parts) == 0 {
				continue
			}
			switch parts[0] {
			case "python", "python3", "pytest", "ruff", "build":
				return true
			}
		}
	}
	return false
}

func hasPythonInstall(commands []string) bool {
	for _, command := range commands {
		for _, segment := range acceptanceCommandSegments(command) {
			parts := strings.Fields(segment)
			if len(parts) >= 4 && (parts[0] == "python" || parts[0] == "python3") && parts[1] == "-m" && parts[2] == "pip" && parts[3] == "install" {
				return true
			}
		}
	}
	return false
}

func acceptanceCommandSegments(command string) []string {
	segments := strings.FieldsFunc(command, func(r rune) bool { return r == ';' || r == '&' || r == '|' })
	if len(segments) == 0 {
		return []string{command}
	}
	return segments
}

func acceptanceCommandParts(command, pythonBin string) []string {
	parts := strings.Fields(command)
	if len(parts) == 0 || pythonBin == "" {
		return parts
	}
	venvBin := filepath.Dir(pythonBin)
	switch parts[0] {
	case "python", "python3":
		parts[0] = pythonBin
	case "pytest", "ruff", "build":
		parts[0] = filepath.Join(venvBin, parts[0])
	}
	return parts
}

func compactFailureLogs(logs string, maxBytes int) string {
	logs = strings.TrimSpace(logs)
	if maxBytes <= 0 || len(logs) <= maxBytes {
		return logs
	}
	return "[truncated]\n" + logs[len(logs)-maxBytes:]
}

func workerCostComponent(model, usageText string, runtimeSeconds float64) cost.Component {
	component := cost.Component{
		Kind:                 "worker",
		ModelRequested:       model,
		UsageSource:          cost.Estimated,
		PricingVersion:       cost.PricingVersion,
		PricingEffectiveDate: cost.PricingEffectiveDate,
		PricingSource:        cost.PricingSource,
		RuntimeSeconds:       runtimeSeconds,
		MachineType:          "e2-small",
		ProvisioningModel:    "STANDARD",
	}
	if usage, err := cost.ParseGeminiCLIStreamJSONL([]byte(usageText), model); err == nil {
		component.ModelRequested = usage.ModelRequested
		component.ModelVersion = usage.ModelVersion
		component.UsageSource = usage.UsageSource
		component.InputTokens = usage.InputTokens
		component.CachedInputTokens = usage.CachedInputTokens
		component.FreshInputTokens = usage.FreshInputTokens
		component.OutputTokens = usage.OutputTokens
		component.ThinkingTokens = usage.ThinkingTokens
		component.TotalTokens = usage.TotalTokens
		component.ProviderResponseID = usage.ProviderResponseID
		component.ProviderRequestID = usage.ProviderRequestID
		pt := cost.PriceForModel(model)
		if pt.Known {
			component.ModelCostUSD = cost.ModelCost(pt, usage.FreshInputTokens, usage.CachedInputTokens, usage.OutputTokens)
		} else {
			component.UnmeteredReason = appendCostReason(component.UnmeteredReason, "model pricing is unknown for "+model)
		}
	} else {
		component.UnmeteredReason = appendCostReason(component.UnmeteredReason, "Gemini CLI provider telemetry unavailable: "+err.Error())
	}
	computeCost, known := cost.ComputeCost(component.MachineType, component.ProvisioningModel, runtimeSeconds)
	component.ComputeCostUSD = computeCost
	if !known {
		component.UnmeteredReason = appendCostReason(component.UnmeteredReason, "Batch compute pricing is unknown")
	} else {
		component.UnmeteredReason = appendCostReason(component.UnmeteredReason, "Batch compute is estimated from observed runtime; invoice data was not queried")
	}
	component.TotalCostUSD = component.ModelCostUSD + component.ComputeCostUSD
	return component
}

func appendCostReason(existing, addition string) string {
	if existing == "" {
		return addition
	}
	return existing + "; " + addition
}

func saveRunResult(cfg *config.Config, result jobs.RunResult) {
	dir := filepath.Join(cfg.ResultsDir, "runs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, result.JobID+".json"), data, 0o644)
}

func writeLedger(cfg *config.Config, result jobs.RunResult, model string) {
	_ = AppendLedger(cfg.ResultsDir, MakeLedgerEntry(result, model))
}
