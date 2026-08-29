package main

import (
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
	"github.com/matthewjameswatkins1978-cyber/gary/internal/gitops"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/jobs"
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
}

type masterPlan struct {
	Done    bool            `json:"done"`
	Summary string          `json:"summary"`
	Jobs    []masterPlanJob `json:"jobs"`
}

type masterPlanJob struct {
	ID         string   `json:"id"`
	Task       string   `json:"task"`
	Acceptance []string `json:"acceptance"`
}

type masterWaveRecord struct {
	Wave          int              `json:"wave"`
	PlannerBranch string           `json:"planner_branch,omitempty"`
	PlanSummary   string           `json:"plan_summary,omitempty"`
	Results       []jobs.RunResult `json:"results,omitempty"`
	Integrated    []string         `json:"integrated,omitempty"`
}

type masterStatus struct {
	ID            string             `json:"id"`
	State         string             `json:"state"`
	UpdatedAt     string             `json:"updated_at"`
	StagingBranch string             `json:"staging_branch,omitempty"`
	Wave          int                `json:"wave,omitempty"`
	Summary       string             `json:"summary,omitempty"`
	Error         string             `json:"error,omitempty"`
	Waves         []masterWaveRecord `json:"waves,omitempty"`
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
	if prior, err := readMasterStatus(manifest.ID); err == nil && isFinalMasterState(prior.State) {
		fmt.Printf("GARY // MASTER ALREADY %s\nJob: %s\nBranch: %s\n", strings.ToUpper(prior.State), prior.ID, prior.StagingBranch)
		return
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

	status := &masterStatus{ID: manifest.ID, State: "starting"}
	stagingBranch := fmt.Sprintf("gary/master-%s-%s", shortMasterID(manifest.ID, 16), randomSuffix(6))
	status.StagingBranch = stagingBranch
	_ = saveMasterStatus(status)

	if err := createMasterStagingBranch(manifest.Repo, manifest.BaseBranch, stagingBranch); err != nil {
		failMaster(status, fmt.Errorf("create staging branch: %w", err))
	}

	fmt.Printf("GARY // MASTER\nJob:       %s\nStaging:   %s\nPlanner:   %s\n", manifest.ID, stagingBranch, cfg.MasterModel)

	for wave := 1; wave <= manifest.MaxWaves; wave++ {
		status.State = "planning"
		status.Wave = wave
		_ = saveMasterStatus(status)

		history, _ := json.Marshal(status.Waves)
		planPath := fmt.Sprintf(".gary/master/%s-wave-%d.json", manifest.ID, wave)
		planJob := &jobs.Job{
			ID:         masterRuntimeID("plan", manifest.ID, wave, 0),
			Repo:       manifest.Repo,
			BaseBranch: stagingBranch,
			Task:       buildMasterPlannerPrompt(manifest, brief, stagingBranch, wave, string(history), planPath),
			Acceptance: []string{"python -m json.tool " + planPath},
		}

		planResult := RunJobOnceWithModel(cfg, planJob, "yardmaster", cfg.MasterModel)
		record := masterWaveRecord{Wave: wave, PlannerBranch: planResult.Branch}
		if planResult.FinalStatus != "DONE" {
			status.Waves = append(status.Waves, record)
			failMaster(status, fmt.Errorf("planner failed in wave %d: %s", wave, planResult.FinalStatus))
		}

		plan, err := readMasterPlan(manifest.Repo, planResult.Branch, planPath, manifest.MaxJobsPerWave)
		if err != nil {
			status.Waves = append(status.Waves, record)
			failMaster(status, fmt.Errorf("read planner output for wave %d: %w", wave, err))
		}
		record.PlanSummary = plan.Summary
		status.Summary = plan.Summary

		if plan.Done {
			status.Waves = append(status.Waves, record)
			if len(manifest.Acceptance) > 0 {
				status.State = "final-verification"
				_ = saveMasterStatus(status)
				if err := runMasterAcceptance(manifest.Repo, stagingBranch, manifest.Acceptance); err != nil {
					failMaster(status, fmt.Errorf("final acceptance: %w", err))
				}
			}
			status.State = "done"
			status.Summary = plan.Summary
			_ = saveMasterStatus(status)
			fmt.Printf("GARY // MASTER COMPLETE\nBranch: %s\nWaves: %d\n", stagingBranch, wave)
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
			childJobs = append(childJobs, &jobs.Job{
				ID:         masterRuntimeID("job", manifest.ID, wave, i+1),
				Repo:       manifest.Repo,
				BaseBranch: stagingBranch,
				Task:       child.Task,
				Acceptance: child.Acceptance,
			})
		}

		results := runMasterWave(cfg, childJobs, manifest.MaxWorkers)
		record.Results = results
		status.State = "integrating"
		_ = saveMasterStatus(status)

		for _, result := range results {
			if result.FinalStatus != "DONE" {
				continue
			}
			if err := integrateMasterResult(manifest.Repo, stagingBranch, result); err != nil {
				status.Waves = append(status.Waves, record)
				failMaster(status, fmt.Errorf("integrate %s: %w", result.JobID, err))
			}
			record.Integrated = append(record.Integrated, result.JobID)
		}

		status.Waves = append(status.Waves, record)
		_ = saveMasterStatus(status)
	}

	failMaster(status, fmt.Errorf("master job reached max_waves=%d without completion", manifest.MaxWaves))
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
	if !strings.HasPrefix(m.Repo, "https://github.com/matthewjameswatkins1978-cyber/") || !strings.HasSuffix(m.Repo, ".git") {
		return nil, "", errors.New("repo must be an HTTPS repository under matthewjameswatkins1978-cyber")
	}
	if !inboxBranch.MatchString(m.BaseBranch) || strings.Contains(m.BaseBranch, "..") {
		return nil, "", errors.New("invalid base_branch")
	}
	if strings.TrimSpace(m.BriefPath) == "" {
		return nil, "", errors.New("brief_path is required")
	}
	brief, err := os.ReadFile(m.BriefPath)
	if err != nil {
		return nil, "", fmt.Errorf("read brief_path: %w", err)
	}
	if len(brief) == 0 {
		return nil, "", errors.New("master brief is empty")
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
	if m.MaxWorkers == 0 {
		m.MaxWorkers = masterWorkerLimit()
	}
	if m.MaxWorkers < 1 || m.MaxWorkers > 8 {
		return nil, "", errors.New("max_workers must be between 1 and 8")
	}
	for _, command := range m.Acceptance {
		if !inboxCommand.MatchString(command) {
			return nil, "", fmt.Errorf("final acceptance command %q contains unsupported shell syntax", command)
		}
	}
	return &m, string(brief), nil
}

func buildMasterPlannerPrompt(m *masterManifest, brief, stagingBranch string, wave int, history, planPath string) string {
	return fmt.Sprintf(`You are GARY Yardmaster, the planning and supervision layer above bounded implementation workers.

You are NOT the implementation worker. Inspect the repository at the current branch, read the complete master brief below, and decide the smallest useful next wave of work.

Rules:
- Do not redesign the project unless the master brief explicitly requires it.
- Prefer finishing and verification over cleanup.
- Create at most %d independent jobs that can safely start from the same current branch.
- Each job must be bounded and have one or more direct executable acceptance commands.
- Do not use shell operators such as &&, |, ;, redirects, or command substitution in acceptance commands.
- Jobs may inspect or modify the repository as needed, but keep scopes non-overlapping where practical.
- If prior workers failed, replan around the evidence rather than repeating them blindly.
- Set done=true only when the master objective is genuinely satisfied in the current repository state.
- The planning branch is disposable. Do not change product files. Write only the JSON plan file at %s.

Required JSON schema:
{
  "done": false,
  "summary": "short state judgement",
  "jobs": [
    {
      "id": "human-readable-short-id",
      "task": "complete bounded worker instruction",
      "acceptance": ["direct command", "another direct command"]
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
`, m.MaxJobsPerWave, planPath, m.ID, wave, m.Repo, stagingBranch, history, brief)
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
		if strings.TrimSpace(child.Task) == "" || len(child.Task) > 12000 {
			return fmt.Errorf("job %d task is empty or too large", i+1)
		}
		if len(child.Acceptance) == 0 {
			return fmt.Errorf("job %d has no acceptance commands", i+1)
		}
		for _, command := range child.Acceptance {
			if !inboxCommand.MatchString(command) {
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
		parts := strings.Fields(command)
		cmd := exec.Command(parts[0], parts[1:]...)
		cmd.Dir = tmp
		cmd.Env = os.Environ()
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s failed: %w\n%s", command, err, strings.TrimSpace(string(out)))
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
