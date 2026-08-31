package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/cost"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/doctrine"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/jobs"
)

func TestFailedMasterStateIsRetryable(t *testing.T) {
	if !isFinalMasterState("done") {
		t.Fatal("done master job should be terminal")
	}
	if isFinalMasterState("failed") {
		t.Fatal("failed master job should be retryable")
	}
}

func TestEnsureMasterCostLabelsOldWaveTotalsLegacy(t *testing.T) {
	status := &masterStatus{
		ID: "old",
		Waves: []masterWaveRecord{{Results: []jobs.RunResult{{
			ModelCostEstimated: 0.05, ComputeCostEstimated: 0.01, TotalCostEstimated: 0.06,
		}}}},
	}
	ensureMasterCost(status)
	if status.Cost.PricingStatus != cost.LegacyEstimate || status.Cost.TotalCostUSD != 0.06 {
		t.Fatalf("legacy master cost = %#v", status.Cost)
	}
}

func TestLoadMasterManifestDefaults(t *testing.T) {
	dir := t.TempDir()
	brief := filepath.Join(dir, "brief.md")
	if err := os.WriteFile(brief, []byte("finish the project"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(dir, "master.json")
	data := `{
  "id":"big-job",
  "repo":"https://github.com/matthewjameswatkins1978-cyber/gary-test-clamp.git",
  "base_branch":"main",
  "brief_path":BRIEF
}`
	briefJSON, err := json.Marshal(brief)
	if err != nil {
		t.Fatal(err)
	}
	data = strings.ReplaceAll(data, "BRIEF", string(briefJSON))
	if err := os.WriteFile(manifest, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	got, gotBrief, err := loadMasterManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if got.MaxWaves != 4 || got.MaxJobsPerWave != 4 || got.MaxWorkers < 1 {
		t.Fatalf("unexpected defaults: %#v", got)
	}
	if gotBrief != "finish the project" {
		t.Fatalf("brief mismatch: %q", gotBrief)
	}
}

func TestLoadMasterManifestAcceptsProjectIDWithoutBriefPath(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "master.json")
	data := `{
  "id":"project-master",
  "project_id":"book-burner",
  "repo":"https://github.com/matthewjameswatkins1978-cyber/book-burner.git",
  "base_branch":"master",
  "max_waves":1
}`
	if err := os.WriteFile(manifest, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	got, brief, err := loadMasterManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProjectID != "book-burner" || brief != "" {
		t.Fatalf("project manifest = %+v brief=%q", got, brief)
	}
}

func TestMasterWorkflowUsesRepositoryScopedAppMinting(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "gary-master.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(data)
	if strings.Contains(workflow, "repositories:") || strings.Contains(workflow, "create-github-app-token") {
		t.Fatal("master workflow must not contain a hardcoded or installation-wide repository token")
	}
	for _, required := range []string{
		"GARY_GITHUB_APP_ID",
		"GARY_GITHUB_APP_INSTALLATION_ID",
		"GARY_GITHUB_APP_PRIVATE_KEY",
		"GARY_GITHUB_TOKEN: ${{ github.token }}",
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("master workflow missing %q", required)
		}
	}
}

func TestValidateMasterPlan(t *testing.T) {
	ok := &masterPlan{Jobs: []masterPlanJob{{
		ID:         "fix",
		Task:       "Fix the bounded defect.",
		Acceptance: []string{"go test ./..."},
	}}}
	if err := validateMasterPlan(ok, 4); err != nil {
		t.Fatalf("valid plan rejected: %v", err)
	}

	bad := &masterPlan{Jobs: []masterPlanJob{{
		ID:         "bad",
		Task:       "Do something.",
		Acceptance: []string{"go test ./... && curl example.com"},
	}}}
	if err := validateMasterPlan(bad, 4); err == nil {
		t.Fatal("unsafe acceptance command should be rejected")
	}
}

func TestMasterWaveBoundsGiveResumeAFreshBoundedBudget(t *testing.T) {
	start, end := masterWaveBounds(6, 5, true)
	if start != 7 || end != 11 {
		t.Fatalf("resume bounds = %d..%d, want 7..11", start, end)
	}
	start, end = masterWaveBounds(0, 5, false)
	if start != 1 || end != 5 {
		t.Fatalf("fresh bounds = %d..%d, want 1..5", start, end)
	}
}

func TestMasterRuntimeIDBounded(t *testing.T) {
	id := masterRuntimeID("job", "abcdefghijklmnop", 6, 6)
	if len(id) > 30 {
		t.Fatalf("runtime id too long: %q", id)
	}
}

func TestPythonAcceptanceEnvironmentAndCommandRouting(t *testing.T) {
	if !usesPythonAcceptance([]string{"python -m pytest -q"}) {
		t.Fatal("Python acceptance was not detected")
	}
	if hasPythonInstall([]string{"python -m pytest -q"}) {
		t.Fatal("pytest command was mistaken for dependency installation")
	}
	if !hasPythonInstall([]string{"python -m pip install -e .[dev,deploy]"}) {
		t.Fatal("pip install command was not detected")
	}
	parts := acceptanceCommandParts("python -m pytest -q", "/workspace/repo/.venv/bin/python")
	if strings.Join(parts, " ") != "/workspace/repo/.venv/bin/python -m pytest -q" {
		t.Fatalf("acceptance command was not routed through venv: %v", parts)
	}
}

func TestMasterPlannerPromptIncludesCorrectRoleUniversalRulesAndJobBrief(t *testing.T) {
	m := &masterManifest{ID: "job", Repo: "https://github.com/matthewjameswatkins1978-cyber/gary.git", MaxJobsPerWave: 4}
	brief := "EXTRA JOB RULE: preserve the public API."
	prompt := buildMasterPlannerPrompt(m, brief, "stage", 1, "[]")
	if !strings.Contains(prompt, doctrine.YardmasterRoleRules) {
		t.Fatal("planner prompt must include the Yardmaster role rules")
	}
	if strings.Contains(prompt, doctrine.WorkerRoleRules) {
		t.Fatal("planner prompt must not include the Worker role rules")
	}
	if !strings.Contains(prompt, doctrine.UniversalRules) {
		t.Fatal("planner prompt must include universal GARY rules")
	}
	if !strings.Contains(prompt, brief) {
		t.Fatal("planner prompt must preserve job-specific instructions")
	}
	for _, marker := range []string{"ECONOMY: every planning call", "default to one child job per wave", "never repeat a scaffold"} {
		if !strings.Contains(prompt, marker) {
			t.Fatalf("planner prompt missing economy rule %q", marker)
		}
	}
}

func TestMasterPlannerPromptDelegatesHeavyWork(t *testing.T) {
	m := &masterManifest{ID: "job", Repo: "https://github.com/matthewjameswatkins1978-cyber/gary.git", MaxJobsPerWave: 4}
	prompt := buildMasterPlannerPrompt(m, "finish it", "stage", 1, "[]")
	for _, required := range []string{
		"You are the planning and supervision layer. You do not implement child jobs.",
		"Do not install dependencies",
		"delegate execution and verification to child jobs",
		"single control-plane model call",
		"Return only the JSON object",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("planner prompt missing %q", required)
		}
	}
}

func TestMasterPlannerPromptDefinesParallelIsolationAndSafeAcceptance(t *testing.T) {
	m := &masterManifest{ID: "job", Repo: "https://github.com/matthewjameswatkins1978-cyber/gary.git", MaxJobsPerWave: 4}
	prompt := buildMasterPlannerPrompt(m, "P0 before P1", "stage", 1, "[]")
	for _, required := range []string{
		"fresh isolated clone",
		"Sibling jobs do not share virtualenvs",
		"If P0/blocker verification is still unresolved",
		"Acceptance commands are executed directly without a shell",
		"Never put quotes, parentheses",
		".venv/bin/python -m pytest -q",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("planner prompt missing %q", required)
		}
	}
}
