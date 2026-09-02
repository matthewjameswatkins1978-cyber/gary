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
	if isFinalMasterState("paused") {
		t.Fatal("paused master job should be resumable")
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

func TestMasterRequiresIndependentFinalSemanticInspection(t *testing.T) {
	masterData, err := os.ReadFile("master.go")
	if err != nil {
		t.Fatal(err)
	}
	plannerData, err := os.ReadFile("planner.go")
	if err != nil {
		t.Fatal(err)
	}
	masterSource := string(masterData)
	plannerSource := string(plannerData)
	for _, required := range []string{"runFinalSemanticInspection", "final-inspection", "REJECTED completion"} {
		if !strings.Contains(masterSource, required) {
			t.Fatalf("master completion path missing semantic inspection marker %q", required)
		}
	}
	for _, required := range []string{"independent final acceptance inspector", "Do not trust Builder or Yardmaster completion claims as evidence", "A dependency added without the requested implementation is not completion"} {
		if !strings.Contains(plannerSource, required) {
			t.Fatalf("final inspector prompt missing %q", required)
		}
	}
}

func TestMasterWorkflowAutoContinuesPausedRunsAndPersistsProjectContext(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "gary-master.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(data)
	for _, required := range []string{
		"GARY_MASTER_RUN_SLICES",
		`paused)`,
		"gary/projects",
		"Continuing paused master",
		"Project terminality is monotonic",
		"Preserving terminal/newer remote project state",
		`remote_state == "ARCHIVED"`,
		`remote_state == "ACCEPTED"`,
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("master workflow missing long-project continuity marker %q", required)
		}
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
		ID:               "fix",
		Task:             "Fix the bounded defect.",
		Acceptance:       []string{"go test ./..."},
		EstimatedMinutes: 5,
	}}}
	if err := validateMasterPlan(ok, 4); err != nil {
		t.Fatalf("valid plan rejected: %v", err)
	}

	bad := &masterPlan{Jobs: []masterPlanJob{{
		ID:               "bad",
		Task:             "Do something.",
		Acceptance:       []string{"go test ./... ; curl example.com"},
		EstimatedMinutes: 5,
	}}}
	if err := validateMasterPlan(bad, 4); err == nil {
		t.Fatal("unsafe acceptance command should be rejected")
	}

	for _, estimate := range []int{0, 61} {
		invalidEstimate := &masterPlan{Jobs: []masterPlanJob{{
			ID: "bad-estimate", Task: "Do something bounded.", Acceptance: []string{"go test ./..."}, EstimatedMinutes: estimate,
		}}}
		if err := validateMasterPlan(invalidEstimate, 4); err == nil {
			t.Fatalf("estimate %d should be rejected", estimate)
		}
	}
}

func TestValidateMasterPlanAllowsOrdinaryShellVerifierComposition(t *testing.T) {
	plan := &masterPlan{Jobs: []masterPlanJob{{
		ID: "docs", Task: "Verify required documentation exists.",
		Acceptance: []string{
			"test -f README.md && test -f docs/ARCHITECTURE.md && test -f docs/PROTOCOL.md",
			"go run ./cmd/telltail version | grep -Eiq 'telltail.*0\\.3'",
			"tmpd=\"$(mktemp -d)\"; test -d \"$tmpd\"",
			"GOOS=windows GOARCH=amd64 go build ./cmd/telltail",
		},
		EstimatedMinutes: 12,
	}}}
	if err := validateMasterPlan(plan, 4); err != nil {
		t.Fatalf("ordinary safe shell acceptance was rejected: %v", err)
	}
}

func TestValidateAcceptanceRejectsDangerousControlPlaneActions(t *testing.T) {
	for _, command := range []string{
		"go test ./... ; curl example.com",
		"git push origin main",
		"sudo apt-get install something",
		"rm -rf /",
	} {
		if validateAcceptanceCommand(command) {
			t.Fatalf("dangerous acceptance command was allowed: %q", command)
		}
	}
}

func TestMasterPlanJSONCarriesWorkerEstimate(t *testing.T) {
	original := masterPlan{Summary: "one bounded slice", Jobs: []masterPlanJob{{
		ID: "slice", Task: "Make one small change.", Acceptance: []string{"go test ./..."}, EstimatedMinutes: 6,
	}}}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"estimated_minutes":6`) {
		t.Fatalf("plan JSON omitted estimate: %s", data)
	}
	var decoded masterPlan
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Jobs[0].EstimatedMinutes != 6 {
		t.Fatalf("decoded estimate = %d, want 6", decoded.Jobs[0].EstimatedMinutes)
	}
}

func TestValidateIntegrableMasterResult(t *testing.T) {
	good := jobs.RunResult{
		JobID: "good", FinalStatus: "DONE", Accepted: true,
		Verification: "trusted-independent", QualityReview: "PASS: merge-worthy",
		Branch: "gary/good", CommitSHA: "abc123",
	}
	if err := validateIntegrableMasterResult(good); err != nil {
		t.Fatalf("fully accepted pushed result rejected: %v", err)
	}

	cases := []jobs.RunResult{
		{JobID: "done-unaccepted", FinalStatus: "DONE", Accepted: false, Branch: "gary/x", CommitSHA: "abc123"},
		{JobID: "accepted-failed", FinalStatus: "FAILED", Accepted: true, Verification: "trusted-independent", QualityReview: "PASS", Branch: "gary/x", CommitSHA: "abc123"},
		{JobID: "accepted-no-verification", FinalStatus: "DONE", Accepted: true, QualityReview: "PASS", Branch: "gary/x", CommitSHA: "abc123"},
		{JobID: "accepted-no-quality", FinalStatus: "DONE", Accepted: true, Verification: "trusted-independent", Branch: "gary/x", CommitSHA: "abc123"},
		{JobID: "accepted-rejected-quality", FinalStatus: "DONE", Accepted: true, Verification: "trusted-independent", QualityReview: "SMALL_FIX: tidy error path", Branch: "gary/x", CommitSHA: "abc123"},
		{JobID: "accepted-no-branch", FinalStatus: "DONE", Accepted: true, Verification: "trusted-independent", QualityReview: "PASS", CommitSHA: "abc123"},
		{JobID: "accepted-no-commit", FinalStatus: "DONE", Accepted: true, Verification: "trusted-independent", QualityReview: "PASS", Branch: "gary/x"},
	}
	for _, tc := range cases {
		if err := validateIntegrableMasterResult(tc); err == nil {
			t.Fatalf("inconsistent worker result accepted: %+v", tc)
		}
	}

	for _, status := range []string{"FAILED", "BLOCKED", "REPLAN_REQUIRED", "BUDGET_STOPPED"} {
		if err := validateIntegrableMasterResult(jobs.RunResult{JobID: status, FinalStatus: status}); err != nil {
			t.Fatalf("ordinary non-accepted result %s should remain replannable: %v", status, err)
		}
	}
}

func TestChildQualityDisposition(t *testing.T) {
	cases := []struct {
		plan *masterPlan
		want childQualityDisposition
	}{
		{&masterPlan{Done: true, Summary: "PASS: sound"}, childQualityPass},
		{&masterPlan{Done: false, Summary: "SMALL_FIX: one error path"}, childQualitySmallFix},
		{&masterPlan{Done: false, Summary: "MAJOR_FIX: incomplete semantics"}, childQualityMajorFix},
		{&masterPlan{Done: false, Summary: "REPLAN: wrong dependency boundary"}, childQualityReplan},
		{&masterPlan{Done: false, Summary: "unclear rejection"}, childQualityMajorFix},
	}
	for _, tc := range cases {
		if got := classifyChildQuality(tc.plan); got != tc.want {
			t.Fatalf("classifyChildQuality(%q) = %s, want %s", tc.plan.Summary, got, tc.want)
		}
	}
}

func TestYardmasterDoctrineDefinesFullChildHandover(t *testing.T) {
	for _, required := range []string{
		"trusted control layer must rerun",
		"PASS, SMALL_FIX, MAJOR_FIX, or REPLAN",
		"reasonable professional engineering standard",
		"A child is accepted only after trusted mechanical verification passes, Yardmaster quality judgement is PASS",
	} {
		if !strings.Contains(doctrine.YardmasterRoleRules, required) {
			t.Fatalf("Yardmaster doctrine missing handover invariant %q", required)
		}
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
	prompt := buildMasterPlannerPrompt(m, brief, "stage", 1, "[]", "previous compact memory")
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
	for _, marker := range []string{"ECONOMY: every planning call", "exactly one child job may run in a wave", "never repeat a scaffold", "accepted child work is an integration obligation", "BUDGET_STOPPED", "control-plane integrity fault"} {
		if !strings.Contains(prompt, marker) {
			t.Fatalf("planner prompt missing economy rule %q", marker)
		}
	}
	for _, marker := range []string{"estimated_minutes", "pacing/cost estimate, not a deadline", "roughly every five minutes", "actual_seconds"} {
		if !strings.Contains(prompt, marker) {
			t.Fatalf("planner prompt missing time-estimation rule %q", marker)
		}
	}
	for _, marker := range []string{"ROLLING YARDMASTER MEMORY", "CONTEXT COMPACTION CONTRACT", "at most 1600 characters"} {
		if !strings.Contains(prompt, marker) {
			t.Fatalf("planner prompt missing compaction contract %q", marker)
		}
	}
}

func TestCompactMasterHistoryOmitsRawCostTelemetry(t *testing.T) {
	history := compactMasterHistory([]masterWaveRecord{{
		Wave:        1,
		PlanSummary: strings.Repeat("x", 1400),
		Results:     []jobs.RunResult{{JobID: "job-1", FinalStatus: "DONE", Accepted: true, Verification: "trusted-independent", QualityReview: "PASS: merge-worthy", Attempts: 1, CommitSHA: "abc123", TotalCostEstimated: 0.12, EstimatedMinutes: 5, DurationSecs: 312}},
	}})
	if len(history) >= 1400 {
		t.Fatalf("compact history remained too large: %d bytes", len(history))
	}
	for _, marker := range []string{"\"job_id\":\"job-1\"", "\"status\":\"DONE\"", "\"commit\":\"abc123\"", "\"cost_usd\":0.12", "\"estimated_minutes\":5", "\"actual_seconds\":312"} {
		if !strings.Contains(history, marker) {
			t.Fatalf("compact history missing %q: %s", marker, history)
		}
	}
	if strings.Contains(history, "input_tokens") || strings.Contains(history, "cached_input_tokens") {
		t.Fatalf("compact history leaked raw token telemetry: %s", history)
	}
}

func TestCompactMasterHistoryPreservesWorkerFailureEvidence(t *testing.T) {
	history := compactMasterHistory([]masterWaveRecord{{
		Wave: 2,
		Results: []jobs.RunResult{{
			JobID: "job-2", FinalStatus: "BLOCKED", BatchJobID: "batch-2",
			PatchBytes: 6972, FailureReason: "acceptance failed: cargo not found",
		}},
	}})
	for _, marker := range []string{
		`"batch_job_id":"batch-2"`,
		`"patch_bytes":6972`,
		`"failure_reason":"acceptance failed: cargo not found"`,
	} {
		if !strings.Contains(history, marker) {
			t.Fatalf("compact history missing %q: %s", marker, history)
		}
	}
}

func TestMasterPlannerPromptDelegatesHeavyWork(t *testing.T) {
	m := &masterManifest{ID: "job", Repo: "https://github.com/matthewjameswatkins1978-cyber/gary.git", MaxJobsPerWave: 4}
	prompt := buildMasterPlannerPrompt(m, "finish it", "stage", 1, "[]", "")
	for _, required := range []string{
		"You are the planning and supervision layer. You do not implement child jobs.",
		doctrine.CompletionPrinciple,
		"Give each Worker the outcome, relevant context, constraints, tool or toolchain assumptions",
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
	prompt := buildMasterPlannerPrompt(m, "P0 before P1", "stage", 1, "[]", "")
	for _, required := range []string{
		"fresh isolated clone",
		"Sibling jobs do not share virtualenvs",
		"If P0/blocker verification is still unresolved",
		"Ordinary verifier shell composition",
		"Do not put network access, repository pushes, privilege changes, destructive administration",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("planner prompt missing %q", required)
		}
	}
}

func TestCompactPlannerMemoryIsBoundedAndFallsBack(t *testing.T) {
	if got := compactPlannerMemory("", strings.Repeat("x", 2000)); len(got) != 1600 {
		t.Fatalf("fallback memory length = %d, want 1600", len(got))
	}
	if got := compactPlannerMemory("  accepted c123; next: tests  ", "fallback"); got != "accepted c123; next: tests" {
		t.Fatalf("memory was not trimmed or preferred: %q", got)
	}
}

func TestEconomicPlanDefersAdditionalJobs(t *testing.T) {
	plan := enforceEconomicPlan(&masterPlan{Summary: "next slice", Jobs: []masterPlanJob{
		{ID: "one", Task: "first", EstimatedMinutes: 6}, {ID: "two", Task: "second", EstimatedMinutes: 4},
	}})
	if len(plan.Jobs) != 1 || plan.Jobs[0].ID != "one" {
		t.Fatalf("economic plan jobs = %+v, want only first job", plan.Jobs)
	}
	if plan.Jobs[0].EstimatedMinutes != 6 {
		t.Fatalf("economic gate changed estimate to %d, want 6", plan.Jobs[0].EstimatedMinutes)
	}
	if !strings.Contains(plan.Summary, "deferred to later waves") {
		t.Fatalf("economic plan did not record deferral: %q", plan.Summary)
	}
}
