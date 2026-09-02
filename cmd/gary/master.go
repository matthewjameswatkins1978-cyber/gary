package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/config"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/cost"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/doctrine"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/githubapp"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/gitops"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/jobs"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/project"
)

type masterManifest struct {
	ID             string   `json:"id"`
	Repo           string   `json:"repo"`
	BaseBranch     string   `json:"base_branch"`
	BriefPath      string   `json:"brief_path"`
	MaxWaves       int      `json:"max_waves,omitempty"`
	MaxJobsPerWave int      `json:"max_jobs_per_wave,omitempty"`
	MaxWorkers     int      `json:"max_workers,omitempty"`
	Acceptance     []string `json:"acceptance,omitempty"`
	StagingBranch  string   `json:"staging_branch,omitempty"`
	ProjectID      string   `json:"project_id,omitempty"`
}

type masterPlan struct {
	Done    bool            `json:"done"`
	Summary string          `json:"summary"`
	Memory  string          `json:"memory,omitempty"`
	Jobs    []masterPlanJob `json:"jobs"`
}

type masterPlanJob struct {
	ID               string   `json:"id"`
	Task             string   `json:"task"`
	Acceptance       []string `json:"acceptance"`
	EstimatedMinutes int      `json:"estimated_minutes"`
}

const (
	maxEconomicWaveJobs   = 1
	maxPlannerTaskBytes   = 1600
	maxPlannerMemoryBytes = 1600
)

type masterWaveRecord struct {
	Wave                int              `json:"wave"`
	PlannerMode         string           `json:"planner_mode,omitempty"`
	PlannerDurationSecs float64          `json:"planner_duration_secs,omitempty"`
	PlanSummary         string           `json:"plan_summary,omitempty"`
	Results             []jobs.RunResult `json:"results,omitempty"`
	Integrated          []string         `json:"integrated,omitempty"`
}

// masterPlannerHistory is deliberately smaller than masterWaveRecord. The
// Yardmaster needs decisions and outcomes for replanning, not every provider
// token field and cost component from every prior worker attempt.
type masterPlannerHistory struct {
	Wave       int                       `json:"wave"`
	Plan       string                    `json:"plan,omitempty"`
	Results    []masterPlannerHistoryJob `json:"results,omitempty"`
	Integrated []string                  `json:"integrated,omitempty"`
}

type masterPlannerHistoryJob struct {
	JobID            string  `json:"job_id"`
	Status           string  `json:"status"`
	BatchJobID       string  `json:"batch_job_id,omitempty"`
	Attempts         int     `json:"attempts,omitempty"`
	Accepted         bool    `json:"accepted,omitempty"`
	Commit           string  `json:"commit,omitempty"`
	PatchBytes       int64   `json:"patch_bytes,omitempty"`
	FailureReason    string  `json:"failure_reason,omitempty"`
	CostUSD          float64 `json:"cost_usd,omitempty"`
	EstimatedMinutes int     `json:"estimated_minutes,omitempty"`
	ActualSeconds    float64 `json:"actual_seconds,omitempty"`
}

func compactMasterHistory(waves []masterWaveRecord) string {
	history := make([]masterPlannerHistory, 0, len(waves))
	for _, wave := range waves {
		record := masterPlannerHistory{Wave: wave.Wave, Plan: truncatePlannerText(wave.PlanSummary, 1200), Integrated: wave.Integrated}
		for _, result := range wave.Results {
			record.Results = append(record.Results, masterPlannerHistoryJob{
				JobID: result.JobID, Status: result.FinalStatus, BatchJobID: result.BatchJobID, Attempts: result.Attempts,
				Accepted: result.Accepted, Commit: result.CommitSHA, PatchBytes: result.PatchBytes,
				FailureReason: truncatePlannerText(result.FailureReason, 1200), CostUSD: result.TotalCostEstimated,
				EstimatedMinutes: result.EstimatedMinutes, ActualSeconds: result.DurationSecs,
			})
		}
		history = append(history, record)
	}
	b, _ := json.Marshal(history)
	return string(b)
}

func truncatePlannerText(text string, max int) string {
	text = strings.TrimSpace(text)
	if max <= 0 || len(text) <= max {
		return text
	}
	const suffix = "..."
	if max <= len(suffix) {
		return text[:max]
	}
	return text[:max-len(suffix)] + suffix
}

type masterStatus struct {
	ID                   string             `json:"id"`
	ProjectID            string             `json:"project_id,omitempty"`
	MasterPacketDigest   string             `json:"master_packet_digest,omitempty"`
	ProjectManualVersion int                `json:"project_manual_version,omitempty"`
	State                string             `json:"state"`
	UpdatedAt            string             `json:"updated_at"`
	StagingBranch        string             `json:"staging_branch,omitempty"`
	Wave                 int                `json:"wave,omitempty"`
	Summary              string             `json:"summary,omitempty"`
	Error                string             `json:"error,omitempty"`
	PlannerMemory        string             `json:"planner_memory,omitempty"`
	Waves                []masterWaveRecord `json:"waves,omitempty"`
	Cost                 *cost.Summary      `json:"cost,omitempty"`
}

func cmdMaster() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "Usage: gary master <master-job.json>")
		os.Exit(1)
	}

	manifest, brief, err := loadMasterManifest(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "master: %v\n", err)
		os.Exit(1)
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "master config: %v\n", err)
		os.Exit(1)
	}
	if cfg.GCPProject == "" {
		cfg.GCPProject = discoverProject()
	}
	if cfg.GCPProject == "" {
		fmt.Fprintln(os.Stderr, "master: no GCP project: set GARY_GCP_PROJECT")
		os.Exit(1)
	}
	if cfg.GCPRegion == "" {
		cfg.GCPRegion = "europe-west2"
	}
	projectCtx, projectID, err := ensureMasterProject(cfg, manifest, brief)
	if err != nil {
		fmt.Fprintf(os.Stderr, "master project context: %v\n", err)
		os.Exit(1)
	}
	manifest.ProjectID = projectID
	brief = projectCtx.PlannerInputWithSource()
	if prior, err := readMasterStatus(manifest.ID); err == nil && isFinalMasterState(prior.State) {
		fmt.Printf("GARY // MASTER ALREADY %s\nJob: %s\nBranch: %s\n", strings.ToUpper(prior.State), prior.ID, prior.StagingBranch)
		return
	}
	if err := project.DefaultStore().SaveState(projectID, project.ProjectState{ProjectID: projectID, State: "EXECUTING", CurrentPhase: "planning", CurrentStrategy: projectCtx.Manual.CurrentStrategy, LastEvent: "master started"}); err != nil {
		fmt.Fprintf(os.Stderr, "master project state: %v\n", err)
		os.Exit(1)
	}

	status := &masterStatus{ID: manifest.ID, ProjectID: projectID, MasterPacketDigest: projectCtx.Project.MasterPacketDigest, ProjectManualVersion: projectCtx.Project.ProjectManualVersion, State: "starting", Cost: cost.NewSummary()}
	stagingBranch := manifest.StagingBranch
	if stagingBranch == "" {
		stagingBranch = fmt.Sprintf("gary/master-%s-%s", shortMasterID(manifest.ID, 16), randomSuffix(6))
	} else {
		if err := verifyMasterStagingBranch(manifest.Repo, stagingBranch); err != nil {
			failMaster(status, fmt.Errorf("verify existing staging branch %s: %w", stagingBranch, err))
		}
		if prior, err := readMasterStatus(manifest.ID); err == nil && prior.StagingBranch == stagingBranch {
			status = prior
			status.State = "starting"
			status.Error = ""
			ensureMasterCost(status)
		}
	}
	ensureMasterCost(status)
	status.StagingBranch = stagingBranch
	_ = saveMasterStatus(status)

	if manifest.StagingBranch == "" {
		if err := createMasterStagingBranch(manifest.Repo, manifest.BaseBranch, stagingBranch); err != nil {
			failMaster(status, fmt.Errorf("create staging branch: %w", err))
		}
	}

	fmt.Printf("GARY // MASTER\nJob:       %s\nStaging:   %s\nPlanner:   %s\n", manifest.ID, stagingBranch, cfg.MasterModel)

	resuming := manifest.StagingBranch != "" && len(status.Waves) > 0
	startWave, endWave := masterWaveBounds(len(status.Waves), manifest.MaxWaves, resuming)
	for wave := startWave; wave <= endWave; wave++ {
		projectCtx, err = project.DefaultStore().LoadContext(projectID)
		if err != nil {
			failMaster(status, fmt.Errorf("project context before wave %d: %w", wave, err))
		}
		brief = projectCtx.PlannerInputWithSource()
		if err := project.DefaultStore().SaveState(projectID, project.ProjectState{ProjectID: projectID, State: "PLANNING", CurrentPhase: "planning", CurrentStrategy: projectCtx.Manual.CurrentStrategy, LastEvent: fmt.Sprintf("planning wave %d", wave)}); err != nil {
			failMaster(status, fmt.Errorf("project state before wave %d: %w", wave, err))
		}
		status.State = "planning"
		status.Wave = wave
		_ = saveMasterStatus(status)

		history := compactMasterHistory(status.Waves)
		fmt.Printf("[yardmaster] PLAN wave=%d mode=control-direct model=%s limit=%s\n", wave, cfg.MasterModel, yardmasterPlannerTimeout)
		plan, planDuration, plannerCost, err := runYardmasterDirect(cfg, manifest, brief, stagingBranch, wave, string(history), status.PlannerMemory)
		record := masterWaveRecord{
			Wave:                wave,
			PlannerMode:         "control-direct",
			PlannerDurationSecs: planDuration.Seconds(),
		}
		status.Cost.Add(plannerCost)
		if err != nil {
			status.Waves = append(status.Waves, record)
			failMaster(status, fmt.Errorf("planner failed in wave %d after %.1fs: %w", wave, planDuration.Seconds(), err))
		}
		fmt.Printf("[yardmaster] PLAN READY wave=%d duration=%.1fs jobs=%d done=%t\n", wave, planDuration.Seconds(), len(plan.Jobs), plan.Done)
		record.PlanSummary = plan.Summary
		status.Summary = plan.Summary
		status.PlannerMemory = compactPlannerMemory(plan.Memory, plan.Summary)

		if plan.Done {
			status.Waves = append(status.Waves, record)
			if err := project.DefaultStore().SaveState(projectID, project.ProjectState{ProjectID: projectID, State: "VERIFYING", CurrentPhase: "verification", CurrentStrategy: projectCtx.Manual.CurrentStrategy, LastEvent: "planner reports project done"}); err != nil {
				failMaster(status, fmt.Errorf("project state before final acceptance: %w", err))
			}
			if len(manifest.Acceptance) > 0 {
				status.State = "final-verification"
				_ = saveMasterStatus(status)
				if err := runMasterAcceptance(manifest.Repo, stagingBranch, manifest.Acceptance); err != nil {
					failMaster(status, fmt.Errorf("final acceptance: %w", err))
				}
			}
			status.State = "final-inspection"
			_ = saveMasterStatus(status)
			inspection, inspectionCost, err := runFinalSemanticInspection(cfg, manifest, string(projectCtx.Packet), stagingBranch)
			status.Cost.Add(inspectionCost)
			if err != nil {
				failMaster(status, fmt.Errorf("final semantic inspection: %w", err))
			}
			if !inspection.Done {
				status.State = "planning"
				status.Summary = "Independent final inspection rejected completion: " + strings.TrimSpace(inspection.Summary)
				status.PlannerMemory = compactPlannerMemory("FINAL INSPECTOR REJECTED COMPLETION: "+inspection.Memory+"\n"+inspection.Summary, status.PlannerMemory)
				if err := project.DefaultStore().SaveState(projectID, project.ProjectState{ProjectID: projectID, State: "PLANNING", CurrentPhase: "planning", CurrentStrategy: projectCtx.Manual.CurrentStrategy, LastEvent: "final semantic inspection rejected completion"}); err != nil {
					failMaster(status, fmt.Errorf("project state after final inspection rejection: %w", err))
				}
				_ = saveMasterStatus(status)
				fmt.Printf("[inspector] REJECTED completion: %s\n", inspection.Summary)
				continue
			}
			if err := project.DefaultStore().AcceptProject(projectID, "project-level mechanical and independent semantic acceptance passed"); err != nil {
				failMaster(status, fmt.Errorf("project acceptance: %w", err))
			}
			status.State = "done"
			status.Summary = plan.Summary
			_ = saveMasterStatus(status)
			printMasterComplete(status, stagingBranch, wave)
			return
		}
		if len(plan.Jobs) == 0 {
			status.Waves = append(status.Waves, record)
			failMaster(status, fmt.Errorf("planner returned no jobs and done=false in wave %d", wave))
		}

		status.State = "running"
		_ = saveMasterStatus(status)
		childJobs := make([]*jobs.Job, 0, len(plan.Jobs))
		for i, child := range plan.Jobs {
			runtimeID := masterRuntimeID("job", manifest.ID, wave, i+1)
			childContext := projectCtx.ChildTaskForID(child.Task, runtimeID)
			childJobs = append(childJobs, &jobs.Job{
				ID:                   runtimeID,
				Repo:                 manifest.Repo,
				BaseBranch:           stagingBranch,
				Task:                 child.Task,
				Acceptance:           child.Acceptance,
				EstimatedMinutes:     child.EstimatedMinutes,
				ProjectID:            projectID,
				MasterPacketDigest:   projectCtx.Project.MasterPacketDigest,
				ProjectManualVersion: projectCtx.Project.ProjectManualVersion,
				ProjectContext:       childContext,
			})
		}

		results := runMasterWave(cfg, childJobs, manifest.MaxWorkers)
		for i := range results {
			results[i].EstimatedMinutes = plan.Jobs[i].EstimatedMinutes
		}
		record.Results = results
		for _, result := range results {
			if result.Cost != nil {
				for _, component := range result.Cost.Components {
					status.Cost.Add(component)
				}
			} else {
				status.Cost.Add(cost.Component{
					Kind:                 "worker",
					UsageSource:          cost.LegacyEstimate,
					ModelCostUSD:         result.ModelCostEstimated,
					ComputeCostUSD:       result.ComputeCostEstimated,
					TotalCostUSD:         result.TotalCostEstimated,
					PricingVersion:       cost.PricingVersion,
					PricingEffectiveDate: cost.PricingEffectiveDate,
					PricingSource:        cost.PricingSource,
					UnmeteredReason:      "worker result had no structured cost record",
				})
			}
		}
		status.State = "integrating"
		_ = saveMasterStatus(status)

		for _, result := range results {
			if err := validateIntegrableMasterResult(result); err != nil {
				status.Waves = append(status.Waves, record)
				failMaster(status, fmt.Errorf("worker result integrity %s: %w", result.JobID, err))
			}
			if !result.Accepted {
				continue
			}
			if err := integrateMasterResult(manifest.Repo, stagingBranch, result); err != nil {
				status.Waves = append(status.Waves, record)
				failMaster(status, fmt.Errorf("integrate accepted result %s: %w", result.JobID, err))
			}
			record.Integrated = append(record.Integrated, result.JobID)
		}

		status.Waves = append(status.Waves, record)
		_ = saveMasterStatus(status)
		if err := project.DefaultStore().SaveState(projectID, project.ProjectState{ProjectID: projectID, State: "EXECUTING", CurrentPhase: "executing", CurrentStrategy: projectCtx.Manual.CurrentStrategy, LastEvent: fmt.Sprintf("wave %d completed", wave)}); err != nil {
			failMaster(status, fmt.Errorf("project state after wave %d: %w", wave, err))
		}
	}

	pauseMaster(status, projectCtx, manifest.MaxWaves, endWave)
}

func pauseMaster(status *masterStatus, projectCtx project.ProjectContext, waveBudget, endWave int) {
	status.State = "paused"
	status.Error = ""
	status.Summary = fmt.Sprintf("Healthy bounded run paused after %d waves (through wave %d); project remains resumable.", waveBudget, endWave)
	_ = saveMasterStatus(status)

	state := projectCtx.State
	state.State = "PAUSED"
	state.CurrentPhase = "executing"
	state.LastEvent = fmt.Sprintf("bounded master run paused after wave %d", endWave)
	_ = project.DefaultStore().SaveState(projectCtx.Project.ProjectID, state)

	fmt.Printf("GARY // MASTER PAUSED\nBranch: %s\nThrough wave: %d\nReason: bounded run slice completed; resume is safe\n", status.StagingBranch, endWave)
}

func ensureMasterProject(cfg *config.Config, manifest *masterManifest, brief string) (project.ProjectContext, string, error) {
	store := project.DefaultStore()
	if manifest.ProjectID != "" {
		ctx, err := store.LoadContext(manifest.ProjectID)
		if err != nil {
			return project.ProjectContext{}, manifest.ProjectID, err
		}
		if ctx.Project.State == "ACCEPTED" || ctx.Project.State == "ARCHIVED" {
			return project.ProjectContext{}, manifest.ProjectID, fmt.Errorf("project %s is terminal (%s)", manifest.ProjectID, ctx.Project.State)
		}
		return ctx, manifest.ProjectID, err
	}
	projectID := manifest.ID
	if existing, err := store.LoadProject(projectID); err == nil {
		ctx, loadErr := store.LoadContext(projectID)
		if loadErr != nil {
			return project.ProjectContext{}, projectID, loadErr
		}
		if ctx.Project.State == "ACCEPTED" || ctx.Project.State == "ARCHIVED" {
			return project.ProjectContext{}, projectID, fmt.Errorf("project %s is terminal (%s)", projectID, ctx.Project.State)
		}
		if existing.Repo != "" && existing.Repo != manifest.Repo {
			return project.ProjectContext{}, projectID, errors.New("project repository mismatch")
		}
		return ctx, projectID, nil
	}
	ctx, err := store.CreateAndIngest(context.Background(), projectID, projectID, manifest.Repo, manifest.BaseBranch, []byte(brief), func(ctx context.Context, input project.ProjectContext) (project.ProjectManual, error) {
		return generateProjectManual(ctx, cfg, input)
	})
	return ctx, projectID, err
}

func loadMasterManifest(path string) (*masterManifest, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	var m masterManifest
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return nil, "", fmt.Errorf("parse manifest: %w", err)
	}
	if !inboxID.MatchString(m.ID) {
		return nil, "", errors.New("id must use lowercase letters, digits, and hyphens")
	}
	if _, err := githubapp.ParseRepository(m.Repo); err != nil {
		return nil, "", fmt.Errorf("invalid repo: %w", err)
	}
	if !inboxBranch.MatchString(m.BaseBranch) || strings.Contains(m.BaseBranch, "..") {
		return nil, "", errors.New("invalid base_branch")
	}
	if m.StagingBranch != "" && (!inboxBranch.MatchString(m.StagingBranch) || strings.Contains(m.StagingBranch, "..")) {
		return nil, "", errors.New("invalid staging_branch")
	}
	var brief []byte
	if m.ProjectID != "" {
		if !project.IsValidID(m.ProjectID) {
			return nil, "", errors.New("invalid project_id")
		}
	} else {
		if strings.TrimSpace(m.BriefPath) == "" {
			return nil, "", errors.New("brief_path is required")
		}
		brief, err = os.ReadFile(m.BriefPath)
		if err != nil {
			return nil, "", fmt.Errorf("read brief_path: %w", err)
		}
		if len(brief) == 0 {
			return nil, "", errors.New("master brief is empty")
		}
	}
	if m.MaxWaves == 0 {
		m.MaxWaves = 4
	}
	if m.MaxWaves < 1 || m.MaxWaves > 6 {
		return nil, "", errors.New("max_waves must be between 1 and 6")
	}
	if m.MaxJobsPerWave == 0 {
		m.MaxJobsPerWave = 4
	}
	if m.MaxJobsPerWave < 1 || m.MaxJobsPerWave > 6 {
		return nil, "", errors.New("max_jobs_per_wave must be between 1 and 6")
	}
	workerLimit := masterWorkerLimit()
	if m.MaxWorkers == 0 || m.MaxWorkers > workerLimit {
		m.MaxWorkers = workerLimit
	}
	if m.MaxWorkers < 1 || m.MaxWorkers > 8 {
		return nil, "", errors.New("max_workers must be between 1 and 8")
	}
	for _, command := range m.Acceptance {
		if !validateAcceptanceCommand(command) {
			return nil, "", fmt.Errorf("final acceptance command %q contains unsupported shell syntax", command)
		}
	}
	return &m, string(brief), nil
}

func buildMasterPlannerPrompt(m *masterManifest, brief, stagingBranch string, wave int, history, memory string) string {
	return fmt.Sprintf(`You are GARY Yardmaster. Read the compact repository snapshot and complete master brief below, then decide the smallest useful next wave of work.

COMPLETION PRINCIPLE
====================
%s

YARDMASTER ROLE
===============
%s

UNIVERSAL GARY RULES
====================
%s

PLANNER-SPECIFIC RULES
======================
- Planning is deliberation only: do not use tools, run checks, deploy services or invoke billable external APIs during the planning call. Tool setup and execution belong to the Worker/runtime.
- Planning is a single control-plane model call. You cannot run tools or tests. Use the supplied repository snapshot and existing evidence, then delegate execution and verification to child jobs.
- Do not redesign the project unless the master brief explicitly requires it.
- Prefer finishing and verification over cleanup.
- Give each Worker the outcome, relevant context, constraints, tool or toolchain assumptions and direct checks needed to work independently. Let the Worker choose implementation tactics and handle ordinary setup within the job's permissions.
- ECONOMY: every planning call and every worker attempt is paid. Choose the smallest useful next slice, and do not schedule speculative future work.
- ECONOMY: exactly one child job may run in a wave. Put one small, independently verifiable outcome in it; defer all other work to later waves after that result is integrated.
- SUPERVISION: workers are live-supervised by the Yardmaster from observable telemetry. Elapsed time is not a failure condition. Do not split a coherent semantic outcome merely to satisfy a clock.
- SUPERVISION: roughly every five minutes of active work, and at natural checkpoints, the Yardmaster may CONTINUE, GUIDE, STOP_AND_REPLAN, or send the candidate to inspection. Plan work that can benefit from this feedback loop.
- TIME ESTIMATION: every job must include an estimated_minutes value from 1 through 60. This is a pacing/cost estimate, not a deadline. Prefer the smallest coherent independently verifiable outcome; compare estimates with actual_seconds to improve later planning without killing productive work.
- ECONOMY: never repeat a scaffold or setup job merely because an earlier attempt was unsuccessful. If the same objective failed or was budget-stopped, use the evidence to split it into a smaller bounded slice or stop and report the blocker.
- CONTROL FLOW: accepted child work is an integration obligation. An accepted result with a pushed branch and commit is integrated into staging by the control plane before dependent planning continues. Plan from that new staging truth; do not recreate or strand accepted work.
- CONTROL FLOW: interpret non-accepted results from evidence. BLOCKED means distinguish genuine authority/external blockage from solvable setup. REPLAN_REQUIRED means change job shape, dependency order, or strategy. BUDGET_STOPPED, timeout, cancellation, or infrastructure interruption are incomplete/indeterminate, not automatic implementation failure. FAILED should only be retried when the next attempt materially changes based on the failure evidence.
- CONTROL FLOW: repeated equivalent failure with no new evidence means stop and replan. A DONE result without mechanical acceptance, or an accepted result without a recoverable branch and commit, is a control-plane integrity fault rather than success.
- ECONOMY: after an accepted result, plan from the current staging branch and extend it; do not recreate already accepted models, tests, configuration, or project structure.
- Workers should use judgment about ordinary tool setup. Keep ephemeral setup, caches and build products out of repository content and do not make one Worker's disposable environment a dependency for another.
- Create at most %d independent jobs that can safely start from the same current branch.
- Every child job runs in its own fresh isolated clone. Sibling jobs do not share virtualenvs, installed packages, untracked files, generated caches, or other filesystem state.
- Do not create a setup job whose uncommitted/ephemeral state is required by another parallel job. Each child must be independently runnable from the current staging branch.
- Respect dependency and priority ordering in the master brief. If P0/blocker verification is still unresolved, do not schedule P1/P2 or cosmetic work in the same wave unless it is truly independent and cannot mask or complicate P0.
- Each job must be bounded and have one or more direct executable acceptance commands.
- Keep each job task concise and under 1600 characters. It must describe one outcome, a small file scope, and one or two direct acceptance commands.
- Give each job one or two simple direct acceptance checks that can be rerun mechanically in a fresh clone. GARY validates the command form before dispatch; do not work around that validation. Put any necessary setup or exploration in the child task.
- Jobs may inspect or modify the repository as needed, but keep scopes non-overlapping where practical.
- If prior workers failed, replan around the evidence rather than repeating them blindly.
- Set done=true only when the master objective is genuinely satisfied in the current repository state.
- If the snapshot is insufficient, delegate one bounded exploration/verification job rather than inventing facts.
- Return only the JSON object below. Do not wrap it in Markdown.

ROLLING YARDMASTER MEMORY
=========================
This is the previous compact working memory. Treat it as a disposable aid, not as a source of truth:
%s

CONTEXT COMPACTION CONTRACT
===========================
Before selecting jobs, compress the useful current state into the JSON memory field. Keep it under 1600 characters and include only: accepted commits, current repository state, the smallest outstanding slice, concrete blockers, and constraints that must not change. Do not copy the master packet, source files, raw logs, token telemetry, or full worker output into memory. The complete packet and evidence remain durable outside this context window.

Required JSON schema:
{
  "done": false,
  "summary": "short state judgement",
  "memory": "compact rolling state for the next wave, at most 1600 characters",
  "jobs": [
    {
      "id": "human-readable-short-id",
      "task": "complete bounded worker instruction",
      "acceptance": ["direct command", "another direct command"],
      "estimated_minutes": 5
    }
  ]
}

MASTER JOB ID: %s
WAVE: %d
TARGET REPO: %s
CURRENT STAGING BRANCH: %s

PREVIOUS WAVE RESULTS:
%s

MASTER BRIEF:
%s
`, doctrine.CompletionPrinciple, doctrine.YardmasterRoleRules, doctrine.UniversalRules, m.MaxJobsPerWave, memory, m.ID, wave, m.Repo, stagingBranch, history, brief)
}

func compactPlannerMemory(memory, fallback string) string {
	memory = strings.TrimSpace(memory)
	if memory == "" {
		memory = strings.TrimSpace(fallback)
	}
	return truncatePlannerText(memory, maxPlannerMemoryBytes)
}

// enforceEconomicPlan is a final control-plane gate. A planner may still
// return multiple ideas despite the contract; only the first bounded job is
// paid for, and the Yardmaster can schedule the rest after observing progress.
func enforceEconomicPlan(plan *masterPlan) *masterPlan {
	if plan == nil || len(plan.Jobs) <= maxEconomicWaveJobs {
		return plan
	}
	plan.Jobs = plan.Jobs[:maxEconomicWaveJobs]
	plan.Summary = strings.TrimSpace(plan.Summary) + " Additional planned jobs were deferred to later waves by the economy gate."
	return plan
}

func readMasterPlan(repoURL, branch, planPath string, maxJobs int) (*masterPlan, error) {
	tmp, err := os.MkdirTemp("", "gary-master-plan-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	if err := gitops.CloneBranch(repoURL, branch, tmp); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(tmp, filepath.FromSlash(planPath)))
	if err != nil {
		return nil, err
	}
	var plan masterPlan
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&plan); err != nil {
		return nil, fmt.Errorf("parse plan: %w", err)
	}
	if err := validateMasterPlan(&plan, maxJobs); err != nil {
		return nil, err
	}
	return &plan, nil
}

func validateMasterPlan(plan *masterPlan, maxJobs int) error {
	if len(plan.Jobs) > maxJobs {
		return fmt.Errorf("planner returned %d jobs; max is %d", len(plan.Jobs), maxJobs)
	}
	for i, child := range plan.Jobs {
		if strings.TrimSpace(child.Task) == "" || len(child.Task) > maxPlannerTaskBytes {
			return fmt.Errorf("job %d task is empty or too large", i+1)
		}
		if child.EstimatedMinutes < 1 || child.EstimatedMinutes > 60 {
			return fmt.Errorf("job %d estimated_minutes must be between 1 and 60", i+1)
		}
		if len(child.Acceptance) == 0 {
			return fmt.Errorf("job %d has no acceptance commands", i+1)
		}
		for _, command := range child.Acceptance {
			if !validateAcceptanceCommand(command) {
				return fmt.Errorf("job %d acceptance %q contains unsupported shell syntax", i+1, command)
			}
		}
	}
	return nil
}

func createMasterStagingBranch(repoURL, baseBranch, stagingBranch string) error {
	tmp, err := os.MkdirTemp("", "gary-master-stage-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if err := gitops.Clone(repoURL, tmp); err != nil {
		return err
	}
	if err := gitops.Checkout(tmp, baseBranch); err != nil {
		return err
	}
	if err := gitops.CreateBranch(tmp, stagingBranch); err != nil {
		return err
	}
	return gitops.Push(tmp, "origin", stagingBranch)
}

func verifyMasterStagingBranch(repoURL, stagingBranch string) error {
	tmp, err := os.MkdirTemp("", "gary-master-existing-stage-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	return gitops.CloneBranch(repoURL, stagingBranch, tmp)
}

func validateIntegrableMasterResult(result jobs.RunResult) error {
	if result.Accepted {
		if result.FinalStatus != "DONE" {
			return fmt.Errorf("accepted result has final_status %q, want DONE", result.FinalStatus)
		}
		if strings.TrimSpace(result.Branch) == "" {
			return errors.New("accepted result has no pushed branch")
		}
		if strings.TrimSpace(result.CommitSHA) == "" {
			return errors.New("accepted result has no commit SHA")
		}
		return nil
	}
	if result.FinalStatus == "DONE" {
		return errors.New("DONE result was not mechanically accepted")
	}
	return nil
}

func integrateMasterResult(repoURL, stagingBranch string, result jobs.RunResult) error {
	tmp, err := os.MkdirTemp("", "gary-master-integrate-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if err := gitops.CloneBranch(repoURL, stagingBranch, tmp); err != nil {
		return err
	}
	if err := gitops.FetchBranch(tmp, "origin", result.Branch); err != nil {
		return err
	}
	if err := gitops.CherryPick(tmp, result.CommitSHA); err != nil {
		_ = gitops.AbortCherryPick(tmp)
		return err
	}
	return gitops.Push(tmp, "origin", stagingBranch)
}

func runMasterWave(cfg *config.Config, childJobs []*jobs.Job, maxWorkers int) []jobs.RunResult {
	if maxWorkers > len(childJobs) {
		maxWorkers = len(childJobs)
	}
	results := make([]jobs.RunResult, len(childJobs))
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup
	for i, job := range childJobs {
		wg.Add(1)
		go func(idx int, j *jobs.Job) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[idx] = RunJob(cfg, j, j.ID)
		}(i, job)
	}
	wg.Wait()
	return results
}

func runMasterAcceptance(repoURL, branch string, commands []string) error {
	tmp, err := os.MkdirTemp("", "gary-master-final-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if err := gitops.CloneBranch(repoURL, branch, tmp); err != nil {
		return err
	}
	for _, command := range commands {
		for _, segment := range acceptanceCommandSegments(command) {
			parts := strings.Fields(segment)
			if len(parts) == 0 {
				return fmt.Errorf("empty final acceptance segment in %q", command)
			}
			cmd := exec.Command(parts[0], parts[1:]...)
			cmd.Dir = tmp
			cmd.Env = os.Environ()
			out, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("%s failed: %w\n%s", segment, err, strings.TrimSpace(string(out)))
			}
		}
	}
	return nil
}

func readMasterStatus(id string) (*masterStatus, error) {
	path := filepath.Join("gary", "master", "status", id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var status masterStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

func isFinalMasterState(state string) bool {
	// Successful completion is idempotently terminal. Failed master jobs are
	// retryable so a corrected brief, model, or runtime policy can use the same ID.
	return state == "done"
}

func saveMasterStatus(status *masterStatus) error {
	ensureMasterCost(status)
	status.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	path := filepath.Join("gary", "master", "status", status.ID+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func ensureMasterCost(status *masterStatus) {
	if status.Cost != nil {
		return
	}
	status.Cost = cost.NewSummary()
	for _, wave := range status.Waves {
		for _, result := range wave.Results {
			status.Cost.Add(cost.Component{
				Kind:                 "worker",
				UsageSource:          cost.LegacyEstimate,
				ModelCostUSD:         result.ModelCostEstimated,
				ComputeCostUSD:       result.ComputeCostEstimated,
				TotalCostUSD:         result.TotalCostEstimated,
				PricingVersion:       "legacy",
				PricingEffectiveDate: "legacy",
				UnmeteredReason:      "migrated from legacy fixed-token estimate",
			})
		}
	}
	if len(status.Cost.Components) > 0 {
		status.Cost.PricingStatus = cost.LegacyEstimate
	}
}

func printMasterComplete(status *masterStatus, branch string, wave int) {
	ensureMasterCost(status)
	fmt.Printf("GARY // MASTER COMPLETE\nBranch: %s\nWaves: %d\n", branch, wave)
	fmt.Printf("Yardmaster: $%.4f\nWorkers: $%.4f\nCloser/repair: $%.4f\nBatch compute: $%.4f\nTOTAL: $%.4f\nCost quality: %s\nPricing: %s\n",
		status.Cost.YardmasterModelCostUSD,
		status.Cost.WorkerModelCostUSD,
		status.Cost.CloserRepairCostUSD,
		status.Cost.ComputeCostUSD,
		status.Cost.TotalCostUSD,
		status.Cost.PricingStatus,
		status.Cost.PricingVersion)
}

func failMaster(status *masterStatus, err error) {
	status.State = "failed"
	status.Error = err.Error()
	_ = saveMasterStatus(status)
	fmt.Fprintf(os.Stderr, "GARY // MASTER FAILED\n%s\n", err)
	os.Exit(1)
}

func masterWorkerLimit() int {
	if raw := os.Getenv("GARY_MAX_WORKERS"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 8 {
			return n
		}
	}
	return 4
}

func masterWaveBounds(existingWaves, waveBudget int, resuming bool) (int, int) {
	start := existingWaves + 1
	end := waveBudget
	if resuming && existingWaves > 0 {
		end = existingWaves + waveBudget
	}
	return start, end
}

func shortMasterID(id string, max int) string {
	if len(id) <= max {
		return id
	}
	return id[:max]
}

func masterRuntimeID(kind, id string, wave, index int) string {
	base := shortMasterID(id, 16)
	if kind == "plan" {
		return fmt.Sprintf("mplan-%s-%d", base, wave)
	}
	return fmt.Sprintf("%s-w%d-j%d", base, wave, index)
}
