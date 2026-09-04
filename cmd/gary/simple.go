package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/batch"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/config"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/cost"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/gitops"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/jobs"
)

// workManifest is the whole-job front door. It deliberately has no model
// escalation, worker-count, wave, planner, supervisor, or retry settings.
type workManifest struct {
	ID               string   `json:"id"`
	Repo             string   `json:"repo"`
	BaseBranch       string   `json:"base_branch"`
	PacketPath       string   `json:"packet_path,omitempty"`
	Task             string   `json:"task,omitempty"`
	Acceptance       []string `json:"acceptance"`
	EstimatedMinutes int      `json:"estimated_minutes,omitempty"`
	MaxRunSeconds    int      `json:"max_run_seconds,omitempty"`

	PacketBytes  []byte `json:"-"`
	PacketDigest string `json:"-"`
	PacketSource string `json:"-"`
}

type blockerRecord struct {
	Kind          string `json:"kind"`
	Summary       string `json:"summary"`
	Needed        string `json:"needed"`
	Evidence      string `json:"evidence"`
	UnblockAction string `json:"unblock_action"`
}

func cmdWork() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "Usage: gary work <job.json>")
		os.Exit(1)
	}
	manifestPath := os.Args[2]
	manifest, err := loadWorkManifest(manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "work: %v\n", err)
		os.Exit(1)
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "work config: %v\n", err)
		os.Exit(1)
	}
	if err := persistWorkPacket(cfg.ResultsDir, manifest); err != nil {
		fmt.Fprintf(os.Stderr, "work packet preservation: %v\n", err)
		os.Exit(1)
	}
	if cfg.GCPProject == "" {
		cfg.GCPProject = discoverProject()
	}
	if cfg.GCPProject == "" {
		fmt.Fprintln(os.Stderr, "work: no GCP project: set GARY_GCP_PROJECT")
		os.Exit(1)
	}
	if cfg.GCPRegion == "" {
		cfg.GCPRegion = "europe-west2"
	}

	job := &jobs.Job{
		ID:                 manifest.ID,
		Repo:               manifest.Repo,
		BaseBranch:         manifest.BaseBranch,
		Task:               manifest.Task,
		Acceptance:         manifest.Acceptance,
		EstimatedMinutes:   manifest.EstimatedMinutes,
		MasterPacketDigest: manifest.PacketDigest,
	}
	result := RunSingleAgentJob(cfg, job, manifest.MaxRunSeconds)
	printSingleAgentResult(result, cfg.ImplementerModel)
	if result.FinalStatus != "DONE" {
		os.Exit(2)
	}
}

func loadWorkManifest(path string) (*workManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var m workManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if strings.TrimSpace(m.ID) == "" || strings.TrimSpace(m.Repo) == "" || strings.TrimSpace(m.BaseBranch) == "" {
		return nil, fmt.Errorf("id, repo, and base_branch are required")
	}
	if len(m.Acceptance) == 0 {
		return nil, fmt.Errorf("at least one acceptance command is required")
	}
	if m.PacketPath != "" && strings.TrimSpace(m.Task) != "" {
		return nil, fmt.Errorf("use packet_path or task, not both")
	}
	if m.PacketPath != "" {
		packetPath := m.PacketPath
		if !filepath.IsAbs(packetPath) {
			packetPath = filepath.Join(filepath.Dir(path), packetPath)
		}
		packet, err := os.ReadFile(packetPath)
		if err != nil {
			return nil, fmt.Errorf("read packet: %w", err)
		}
		m.PacketBytes = append([]byte(nil), packet...)
		m.PacketSource = packetPath
		m.Task = string(packet)
	} else {
		m.PacketBytes = []byte(m.Task)
		m.PacketSource = path + "#task"
	}
	if strings.TrimSpace(m.Task) == "" {
		return nil, fmt.Errorf("packet_path or task is required")
	}
	digest := sha256.Sum256(m.PacketBytes)
	m.PacketDigest = fmt.Sprintf("%x", digest[:])
	if m.MaxRunSeconds == 0 {
		m.MaxRunSeconds = 3600
	}
	if m.MaxRunSeconds < 300 || m.MaxRunSeconds > 14400 {
		return nil, fmt.Errorf("max_run_seconds must be between 300 and 14400")
	}
	return &m, nil
}

// RunSingleAgentJob launches exactly one implementation agent. It does not
// supervise it with another model, retry it, create child jobs, or escalate it.
func RunSingleAgentJob(cfg *config.Config, job *jobs.Job, maxRunSeconds int) jobs.RunResult {
	started := time.Now()
	if maxRunSeconds == 0 {
		maxRunSeconds = 3600
	}
	model := cfg.ImplementerModel
	runCost := cost.NewSummary()
	suffix := randomSuffix(8)
	branch := fmt.Sprintf("gary/%s-%s", job.ID, suffix)
	runID := fmt.Sprintf("%s-%s", job.ID, suffix)
	gcsPath := fmt.Sprintf("gs://gary-batch-results-418146339023/runs/%s", runID)
	jobName := fmt.Sprintf("gary-work-%s-%s", shortSimpleID(job.ID, 24), time.Now().Format("20060102-150405"))

	finish := func(result jobs.RunResult) jobs.RunResult {
		result.JobID = job.ID
		result.Branch = branch
		result.MasterPacketDigest = job.MasterPacketDigest
		result.DurationSecs = time.Since(started).Seconds()
		result.Cost = runCost
		result.ModelCostEstimated = runCost.ModelCostUSD
		result.ComputeCostEstimated = runCost.ComputeCostUSD
		result.TotalCostEstimated = runCost.TotalCostUSD
		saveRunResult(cfg, result)
		writeLedger(cfg, result, model)
		return result
	}

	if cfg.MaxCostUSD > 0 {
		estimated := EstimateJobCostForModel(model, 1)
		if allowed, reason := BudgetCheck(cfg.MaxCostUSD, 0, estimated); !allowed {
			return finish(jobs.RunResult{FinalStatus: "BLOCKED", FailureReason: reason})
		}
	}

	githubToken, err := currentBatchGitHubToken(job.Repo)
	if err != nil {
		return finish(jobs.RunResult{FinalStatus: "BLOCKED", FailureReason: "GitHub authentication unavailable: " + err.Error()})
	}

	codingCfg := batch.CodingJobConfig{
		Project:          cfg.GCPProject,
		Region:           cfg.GCPRegion,
		JobName:          jobName,
		RepoURL:          job.Repo,
		BaseBranch:       job.BaseBranch,
		GaryBranch:       branch,
		Task:             job.Task,
		Acceptance:       strings.Join(job.Acceptance, " && "),
		EstimatedMinutes: job.EstimatedMinutes,
		GCSPath:          gcsPath,
		MachineType:      "e2-small",
		Model:            model,
		GitHubToken:      githubToken,
		MaxRunDurSec:     maxRunSeconds,
	}
	batchJobID, err := batch.SubmitCodingJob(codingCfg)
	if err != nil {
		return finish(jobs.RunResult{FinalStatus: "BLOCKED", FailureReason: "Batch submission failed: " + err.Error()})
	}

	status, finishedAt, err := batch.WaitForTerminal(
		cfg.GCPProject,
		cfg.GCPRegion,
		batchJobID,
		10*time.Second,
		time.Duration(maxRunSeconds+900)*time.Second,
	)
	if err != nil {
		return finish(jobs.RunResult{BatchJobID: batchJobID, FinalStatus: "FAILED", FailureReason: "Batch execution could not be observed to completion: " + err.Error()})
	}

	tmpDir, err := os.MkdirTemp("", "gary-single-*")
	if err != nil {
		return finish(jobs.RunResult{BatchJobID: batchJobID, BatchState: status.State(), FinalStatus: "FAILED", FailureReason: err.Error()})
	}
	defer os.RemoveAll(tmpDir)

	usageText := downloadGCSFile(tmpDir, gcsPath+"/usage.jsonl")
	if usageText == "" {
		usageText = extractFromBatchLogs(cfg.GCPProject, cfg.GCPRegion, batchJobID, "GARY_USAGE_B64")
	}
	runCost.Add(workerCostComponent(model, usageText, batch.ObservedRuntimeSeconds(status, started, finishedAt)))

	blockerText := downloadGCSFile(tmpDir, gcsPath+"/blocker.json")
	if blockerText == "" {
		blockerText = extractFromBatchLogs(cfg.GCPProject, cfg.GCPRegion, batchJobID, "GARY_BLOCKER_B64")
	}
	if blocker := parseBlocker(blockerText); blocker != nil {
		reason := fmt.Sprintf("%s: %s; needed: %s; evidence: %s; unblock: %s", blocker.Kind, blocker.Summary, blocker.Needed, blocker.Evidence, blocker.UnblockAction)
		return finish(jobs.RunResult{
			BatchJobID:     batchJobID,
			BatchState:     status.State(),
			ArtifactPrefix: gcsPath,
			Attempts:       1,
			FinalStatus:    "BLOCKED",
			FailureReason:  compactFailureLogs(reason, 4000),
		})
	}

	patchData := downloadGCSFile(tmpDir, gcsPath+"/result.patch")
	if patchData == "" {
		patchData = extractFromBatchLogs(cfg.GCPProject, cfg.GCPRegion, batchJobID, "GARY_PATCH_B64")
	}
	acceptanceOutput := downloadGCSFile(tmpDir, gcsPath+"/acceptance_output.txt")
	if acceptanceOutput == "" {
		acceptanceOutput = extractFromBatchLogs(cfg.GCPProject, cfg.GCPRegion, batchJobID, "GARY_ACCEPTANCE_B64")
	}
	if strings.TrimSpace(patchData) == "" {
		reason := "agent returned no candidate patch"
		if status.State() != "SUCCEEDED" {
			reason = "Batch ended " + status.State() + " and no candidate patch was recovered"
		}
		if strings.TrimSpace(acceptanceOutput) != "" {
			reason += "; acceptance: " + compactFailureLogs(acceptanceOutput, 1200)
		}
		return finish(jobs.RunResult{BatchJobID: batchJobID, BatchState: status.State(), ArtifactPrefix: gcsPath, Attempts: 1, FinalStatus: "FAILED", FailureReason: reason})
	}

	verify := runLocalAcceptanceClassified(tmpDir, job, branch, patchData, "single")
	if !verify.passed {
		state := "FAILED"
		if verify.blocker {
			state = "BLOCKED"
		}
		reason := "independent acceptance failed"
		if verify.err != nil {
			reason += ": " + verify.err.Error()
		}
		return finish(jobs.RunResult{BatchJobID: batchJobID, BatchState: status.State(), ArtifactPrefix: gcsPath, PatchBytes: int64(len(patchData)), Attempts: 1, Verification: "trusted-independent", FinalStatus: state, FailureReason: compactFailureLogs(reason, 4000)})
	}

	workDir := filepath.Join(tmpDir, "repo")
	commitSHA, _ := gitops.CommitSHA(workDir)
	if err := gitops.Push(workDir, "origin", branch); err != nil {
		return finish(jobs.RunResult{BatchJobID: batchJobID, BatchState: status.State(), ArtifactPrefix: gcsPath, CommitSHA: commitSHA, PatchBytes: int64(len(patchData)), Attempts: 1, Verification: "trusted-independent", FinalStatus: "BLOCKED", FailureReason: "GitHub writeback failed: " + err.Error()})
	}

	return finish(jobs.RunResult{
		BatchJobID:     batchJobID,
		BatchState:     status.State(),
		ArtifactPrefix: gcsPath,
		CommitSHA:      commitSHA,
		PatchBytes:     int64(len(patchData)),
		Attempts:       1,
		Accepted:       true,
		Verification:   "trusted-independent",
		QualityReview:  "not-used-single-agent",
		FinalStatus:    "DONE",
	})
}

func parseBlocker(text string) *blockerRecord {
	text = strings.TrimSpace(text)
	if text == "" || text == "{}" {
		return nil
	}
	var b blockerRecord
	if err := json.Unmarshal([]byte(text), &b); err != nil || strings.TrimSpace(b.Kind) == "" || strings.TrimSpace(b.Summary) == "" {
		return nil
	}
	return &b
}

func shortSimpleID(id string, max int) string {
	id = strings.ToLower(id)
	var b strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "job"
	}
	if len(out) > max {
		out = strings.TrimRight(out[:max], "-")
	}
	return out
}

func printSingleAgentResult(result jobs.RunResult, model string) {
	fmt.Println("GARY // SINGLE AGENT")
	fmt.Printf("Model:   %s\n", model)
	fmt.Printf("Status:  %s\n", result.FinalStatus)
	if result.Branch != "" {
		fmt.Printf("Branch:  %s\n", result.Branch)
	}
	if result.CommitSHA != "" {
		fmt.Printf("Commit:  %s\n", result.CommitSHA)
	}
	if result.FailureReason != "" {
		fmt.Printf("Reason:  %s\n", result.FailureReason)
	}
	if result.Cost != nil {
		fmt.Printf("Cost:    $%.4f USD (%s)\n", result.Cost.TotalCostUSD, result.Cost.PricingStatus)
	}
}
