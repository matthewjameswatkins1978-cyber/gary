package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/batch"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/config"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/gitops"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/jobs"
)

func cmdRun() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: gary run <job-file>")
		os.Exit(1)
	}

	jobFile := os.Args[2]

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config load failed: %v\n", err)
		os.Exit(1)
	}

	if cfg.GCPProject == "" {
		cfg.GCPProject = discoverProject()
	}
	if cfg.GCPProject == "" {
		fmt.Fprintln(os.Stderr, "no GCP project: set GARY_GCP_PROJECT")
		os.Exit(1)
	}
	if cfg.GCPRegion == "" {
		cfg.GCPRegion = "europe-west2"
	}
	if cfg.GeminiAPIKey == "" {
		fmt.Fprintln(os.Stderr, "no Gemini API key: set GEMINI_API_KEY")
		os.Exit(1)
	}

	// Load job
	job, err := loadJob(jobFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load job: %v\n", err)
		os.Exit(1)
	}

	// Validate
	if job.ID == "" || job.Repo == "" || job.BaseBranch == "" || job.Task == "" {
		fmt.Fprintln(os.Stderr, "job requires id, repo, base_branch, and task")
		os.Exit(1)
	}
	if len(job.Acceptance) == 0 {
		job.Acceptance = []string{"echo no-acceptance-command"}
	}

	// Generate branch name
	suffix := randomSuffix(8)
	garyBranch := fmt.Sprintf("gary/%s-%s", job.ID, suffix)

	// Unique run ID for GCS path
	runID := fmt.Sprintf("%s-%s", job.ID, suffix)
	gcsPath := fmt.Sprintf("gs://gary-batch-results-418146339023/runs/%s", runID)

	// Build acceptance command string for worker
	acceptanceStr := strings.Join(job.Acceptance, " && ")

	jobName := fmt.Sprintf("gary-run-%s-%s", job.ID, time.Now().Format("20060102-150405"))

	fmt.Println("GARY // RUN")
	fmt.Println()
	fmt.Printf("Job        %s\n", job.ID)
	fmt.Printf("Worker     batch\n")
	fmt.Printf("Branch     %s\n", garyBranch)
	fmt.Printf("Status     WORKING")
	fmt.Println()

	submittedAt := time.Now()

	// Submit coding job
	codingCfg := batch.CodingJobConfig{
		Project:      cfg.GCPProject,
		Region:       cfg.GCPRegion,
		JobName:      jobName,
		RepoURL:      job.Repo,
		BaseBranch:   job.BaseBranch,
		GaryBranch:   garyBranch,
		Task:         job.Task,
		Acceptance:   acceptanceStr,
		GCSPath:      gcsPath,
		MachineType:  "e2-standard-4",
		MaxRunDurSec: 600,
	}

	batchJobID, err := batch.SubmitCodingJob(codingCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nsubmission failed: %v\n", err)
		os.Exit(1)
	}
	_ = batchJobID

	fmt.Println("\nSUBMITTED")

	// Poll for completion
	fmt.Println("\nPOLLING")

	status, _, err := batch.WaitForTerminal(
		cfg.GCPProject, cfg.GCPRegion, jobName,
		10*time.Second, 15*time.Minute,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\npolling failed: %v\n", err)
		os.Exit(1)
	}

	if status.State() != "SUCCEEDED" {
		fmt.Fprintf(os.Stderr, "\njob ended with state: %s\n", status.State())
		os.Exit(1)
	}

	fmt.Println("\nRETRIEVING RESULT")

	// Download patch from GCS
	tmpDir, err := os.MkdirTemp("", "gary-run-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	patchPath := filepath.Join(tmpDir, "result.patch")
	gcsPatchPath := gcsPath + "/result.patch"

	dlCmd := exec.Command("gsutil", "cp", gcsPatchPath, patchPath)
	dlOut, err := dlCmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "download patch failed: %v\n%s", err, string(dlOut))
		os.Exit(1)
	}

	patchData, err := os.ReadFile(patchPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read patch: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  Patch downloaded: %d bytes\n", len(patchData))

	fmt.Println("\nVERIFYING LOCALLY")

	// Clone repo locally
	workDir := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}

	if err := gitops.Clone(job.Repo, workDir); err != nil {
		fmt.Fprintf(os.Stderr, "clone: %v\n", err)
		os.Exit(1)
	}

	// Checkout base branch
	if err := gitops.Checkout(workDir, job.BaseBranch); err != nil {
		fmt.Fprintf(os.Stderr, "checkout base: %v\n", err)
		os.Exit(1)
	}

	// Verify base branch is unchanged
	baseSHA, err := gitops.CommitSHA(workDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "get base SHA: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  Base SHA: %s\n", baseSHA)

	// Create GARY branch
	if err := gitops.CreateBranch(workDir, garyBranch); err != nil {
		fmt.Fprintf(os.Stderr, "create branch: %v\n", err)
		os.Exit(1)
	}

	// Apply patch
	if err := gitops.ApplyPatch(workDir, string(patchData)); err != nil {
		fmt.Fprintf(os.Stderr, "apply patch: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  Patch applied")

	// Run acceptance commands locally
	accepted := true
	for _, cmd := range job.Acceptance {
		fmt.Printf("  Running: %s\n", cmd)
		parts := strings.Fields(cmd)
		c := exec.Command(parts[0], parts[1:]...)
		c.Dir = workDir
		out, err := c.CombinedOutput()
		if err != nil {
			fmt.Printf("    FAIL: %s\n", strings.TrimSpace(string(out)))
			accepted = false
		} else {
			fmt.Println("    PASS")
		}
	}

	if !accepted {
		fmt.Println("\nFAILED")
		fmt.Println("  Acceptance checks failed")
		os.Exit(1)
	}

	// Commit and push
	if err := gitops.CommitAll(workDir, fmt.Sprintf("GARY: %s", job.Task)); err != nil {
		fmt.Fprintf(os.Stderr, "commit: %v\n", err)
		os.Exit(1)
	}

	commitSHA, _ := gitops.CommitSHA(workDir)

	if err := gitops.Push(workDir, "origin", garyBranch); err != nil {
		fmt.Fprintf(os.Stderr, "push: %v\n", err)
		os.Exit(1)
	}

	duration := time.Since(submittedAt)

	fmt.Println()
	fmt.Println("ACCEPTED")
	fmt.Printf("Branch pushed: %s\n", garyBranch)
	fmt.Printf("Commit: %s\n", commitSHA)
	fmt.Printf("Duration: %.0fs\n", duration.Seconds())

	// Save result
	result := jobs.RunResult{
		JobID:        job.ID,
		BatchJobID:   jobName,
		Branch:       garyBranch,
		CommitSHA:    commitSHA,
		Accepted:     true,
		DurationSecs: duration.Seconds(),
		PatchBytes:   int64(len(patchData)),
	}

	if err := saveRunResult(cfg.ResultsDir, result); err != nil {
		fmt.Fprintf(os.Stderr, "save result: %v\n", err)
	}
}

func loadJob(path string) (*jobs.Job, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read job file: %w", err)
	}

	var job jobs.Job
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("parse job file: %w", err)
	}

	return &job, nil
}

func randomSuffix(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)[:n]
}

func saveRunResult(resultsDir string, result jobs.RunResult) error {
	dir := filepath.Join(resultsDir, "runs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	filename := fmt.Sprintf("%s.json", result.JobID)
	path := filepath.Join(dir, filename)

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}
