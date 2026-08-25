package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

const maxAttempts = 3

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

	job, err := loadJob(jobFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load job: %v\n", err)
		os.Exit(1)
	}

	if job.ID == "" || job.Repo == "" || job.BaseBranch == "" || job.Task == "" {
		fmt.Fprintln(os.Stderr, "job requires id, repo, base_branch, and task")
		os.Exit(1)
	}
	if len(job.Acceptance) == 0 {
		job.Acceptance = []string{"echo no-acceptance-command"}
	}

	suffix := randomSuffix(8)
	garyBranch := fmt.Sprintf("gary/%s-%s", job.ID, suffix)
	runID := fmt.Sprintf("%s-%s", job.ID, suffix)
	gcsPath := fmt.Sprintf("gs://gary-batch-results-418146339023/runs/%s", runID)
	acceptanceStr := strings.Join(job.Acceptance, " && ")

	fmt.Println("GARY // RUN")
	fmt.Println()
	fmt.Printf("Job        %s\n", job.ID)
	fmt.Printf("Worker     batch\n")
	fmt.Printf("Branch     %s\n", garyBranch)
	fmt.Println()

	submittedAt := time.Now()

	// State across attempts
	var lastPatch string
	var lastAcceptance string
	var lastFailure string
	var prevGCSPatch string
	progressHistory := make([]string, 0)
	repairCount := 0
	escalated := false
	closerUsed := false
	finalStatus := "FAILED"

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		attemptTask := job.Task
		if attempt > 1 && lastPatch != "" {
			attemptTask = job.Task + "\n\n## PREVIOUS ATTEMPT FAILED\n\n" + lastFailure
		}

		jobName := fmt.Sprintf("gary-run-%s-%d-%s", job.ID, attempt, time.Now().Format("20060102-150405"))
		attemptGCSPath := fmt.Sprintf("%s/attempt-%d", gcsPath, attempt)

		// Fresh per-attempt workspace
		attemptDir, err := os.MkdirTemp("", fmt.Sprintf("gary-attempt-%d-*", attempt))
		if err != nil {
			fmt.Fprintf(os.Stderr, "create attempt dir: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("ATTEMPT %d\n", attempt)

		codingCfg := batch.CodingJobConfig{
			Project:      cfg.GCPProject,
			Region:       cfg.GCPRegion,
			JobName:      jobName,
			RepoURL:      job.Repo,
			BaseBranch:   job.BaseBranch,
			GaryBranch:   garyBranch,
			Task:         attemptTask,
			Acceptance:   acceptanceStr,
			GCSPath:      attemptGCSPath,
			PrevGCSPath:  prevGCSPatch,
			MachineType:  "e2-small",
			MaxRunDurSec: 600,
		}

		_, err = batch.SubmitCodingJob(codingCfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "submission failed: %v\n", err)
			os.RemoveAll(attemptDir)
			os.Exit(1)
		}

		fmt.Println("  SUBMITTED")

		status, _, err := batch.WaitForTerminal(
			cfg.GCPProject, cfg.GCPRegion, jobName,
			10*time.Second, 15*time.Minute,
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  polling failed: %v\n", err)
			os.RemoveAll(attemptDir)
			os.Exit(1)
		}

		batchSucceeded := status.State() == "SUCCEEDED"

		// Download patch and acceptance output
		patchData := downloadGCSFile(attemptDir, attemptGCSPath+"/result.patch")
		acceptOutput := downloadGCSFile(attemptDir, attemptGCSPath+"/acceptance_output.txt")

		if patchData != "" {
			lastPatch = patchData
			prevGCSPatch = attemptGCSPath + "/result.patch"
		}
		if acceptOutput != "" {
			lastAcceptance = acceptOutput
		}

		if batchSucceeded {
			// Worker reported success — run local acceptance
			fmt.Println("  BATCH SUCCEEDED — verifying locally")

			localAccepted := runLocalAcceptance(attemptDir, job, garyBranch, patchData)
			if localAccepted {
				workDir := filepath.Join(attemptDir, "repo")
				commitSHA, _ := gitops.CommitSHA(workDir)
				if err := gitops.Push(workDir, "origin", garyBranch); err != nil {
					fmt.Fprintf(os.Stderr, "  push: %v\n", err)
				}
				duration := time.Since(submittedAt)
				fmt.Println()
				fmt.Println("ACCEPTED")
				fmt.Printf("Branch pushed: %s\n", garyBranch)
				fmt.Printf("Commit: %s\n", commitSHA)
				fmt.Printf("Attempts: %d\n", attempt)
				fmt.Printf("Duration: %.0fs\n", duration.Seconds())
				finalStatus = "DONE"
				os.RemoveAll(attemptDir)
				saveResult(cfg, job, jobName, garyBranch, commitSHA, true, duration, len(patchData), attempt, repairCount, escalated, closerUsed, finalStatus)
				return
			}
			lastFailure = "Local acceptance failed after Batch success.\n" + lastAcceptance
		} else {
			fmt.Printf("  BATCH %s\n", status.State())
			lastFailure = fmt.Sprintf("Batch job ended with state: %s\n", status.State())
			if lastAcceptance != "" {
				lastFailure += "\nWorker acceptance output:\n" + lastAcceptance
			}
		}

		os.RemoveAll(attemptDir)

		// Check progress
		if attempt > 1 {
			improved := checkProgress(progressHistory, lastAcceptance)
			if !improved {
				fmt.Println("  LOW PROGRESS — no meaningful change")
				if attempt == maxAttempts {
					escalated = true
					break
				}
				fmt.Println()
				continue
			}
		}

		progressHistory = append(progressHistory, summarizeAcceptance(lastAcceptance))

		// Generate repair brief
		fmt.Println("  GENERATING REPAIR BRIEF")
		brief, err := generateRepairBrief(cfg, job.Task, acceptanceStr, lastPatch, lastAcceptance, "")
		if err != nil {
			fmt.Fprintf(os.Stderr, "  repair brief failed: %v\n", err)
			lastFailure = fmt.Sprintf("Repair brief generation failed: %v\n\nPrevious failure:\n%s", err, lastFailure)
			fmt.Println()
			continue
		}
		repairCount++
		fmt.Printf("  Cause: %s\n", brief.Cause)
		fmt.Printf("  Next:  %s\n", brief.NextAction)

		lastFailure = formatRepairBrief(brief)
		fmt.Println()
	}

	// Escalation to Closer
	if escalated {
		fmt.Println("ESCALATING TO CLOSER")
		closerDir, err := os.MkdirTemp("", "gary-closer-*")
		if err != nil {
			fmt.Fprintf(os.Stderr, "create closer dir: %v\n", err)
			os.Exit(1)
		}
		defer os.RemoveAll(closerDir)

		closerTask := buildCloserTask(job, lastPatch, lastAcceptance, lastFailure, progressHistory)
		closeResult, err := runCloser(cfg, closerTask, closerDir, job, garyBranch, acceptanceStr, gcsPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  closer failed: %v\n", err)
			finalStatus = "FAILED"
		} else if closeResult {
			workDir := filepath.Join(closerDir, "repo")
			commitSHA, _ := gitops.CommitSHA(workDir)
			gitops.Push(workDir, "origin", garyBranch)
			duration := time.Since(submittedAt)
			fmt.Println()
			fmt.Println("ACCEPTED (CLOSER)")
			fmt.Printf("Branch pushed: %s\n", garyBranch)
			fmt.Printf("Commit: %s\n", commitSHA)
			fmt.Printf("Attempts: %d\n", maxAttempts)
			fmt.Printf("Duration: %.0fs\n", duration.Seconds())
			closerUsed = true
			finalStatus = "DONE"
			saveResult(cfg, job, fmt.Sprintf("closer-%s", job.ID), garyBranch, commitSHA, true, duration, 0, maxAttempts, repairCount, escalated, closerUsed, finalStatus)
			return
		} else {
			finalStatus = "FAILED"
		}
	}

	fmt.Printf("\nfinal_status=%s\n", finalStatus)
	duration := time.Since(submittedAt)
	saveResult(cfg, job, "", garyBranch, "", false, duration, 0, maxAttempts, repairCount, escalated, closerUsed, finalStatus)
}

func downloadGCSFile(tmpDir, gcsPath string) string {
	path := filepath.Join(tmpDir, filepath.Base(gcsPath))
	cmd := exec.Command("gsutil", "cp", gcsPath, path)
	_, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func runLocalAcceptance(tmpDir string, job *jobs.Job, garyBranch, patchData string) bool {
	workDir := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return false
	}

	if err := gitops.Clone(job.Repo, workDir); err != nil {
		return false
	}
	if err := gitops.Checkout(workDir, job.BaseBranch); err != nil {
		return false
	}
	if err := gitops.CreateBranch(workDir, garyBranch); err != nil {
		return false
	}
	if err := gitops.ApplyPatch(workDir, patchData); err != nil {
		return false
	}

	for _, cmd := range job.Acceptance {
		parts := strings.Fields(cmd)
		c := exec.Command(parts[0], parts[1:]...)
		c.Dir = workDir
		out, err := c.CombinedOutput()
		if err != nil {
			fmt.Printf("    LOCAL FAIL: %s\n", strings.TrimSpace(string(out)))
			return false
		}
		fmt.Println("    LOCAL PASS")
	}

	// Commit the applied patch so push has something to push
	if err := gitops.CommitAll(workDir, fmt.Sprintf("GARY: %s", job.Task)); err != nil {
		fmt.Printf("    LOCAL COMMIT FAIL: %v\n", err)
		return false
	}

	return true
}

type repairBrief struct {
	Cause       string `json:"cause"`
	Keep        string `json:"keep"`
	Change      string `json:"change"`
	DoNotChange string `json:"do_not_change"`
	NextAction  string `json:"next_action"`
}

func generateRepairBrief(cfg *config.Config, task, acceptance, patch, acceptOutput, priorBrief string) (*repairBrief, error) {
	prompt := fmt.Sprintf(`You are a code review repair assistant. Analyze the failed implementation attempt and produce a concise repair brief.

ORIGINAL TASK:
%s

ACCEPTANCE CRITERIA:
%s

WORKER PATCH:
%s

ACCEPTANCE OUTPUT:
%s

PRIOR REPAIR BRIEF:
%s

Respond with exactly this JSON format:
{
  "cause": "root cause of the failure in one sentence",
  "keep": "what useful work to preserve",
  "change": "what to change in the next attempt",
  "do_not_change": "what must remain unchanged",
  "next_action": "single next concrete action for the worker"
}`, task, acceptance, patch, acceptOutput, priorBrief)

	return callGeminiJSON(cfg, prompt)
}

func callGeminiJSON(cfg *config.Config, prompt string) (*repairBrief, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", cfg.CloserModel, cfg.GeminiAPIKey)

	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature": 0.2,
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("gemini request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini API %d: %s", resp.StatusCode, string(respBody))
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

	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if len(apiResp.Candidates) == 0 || len(apiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no candidates in response")
	}

	rawText := apiResp.Candidates[0].Content.Parts[0].Text
	rawText = strings.TrimSpace(rawText)

	// Extract JSON from markdown code fences if present
	if idx := strings.Index(rawText, "```json"); idx != -1 {
		rawText = rawText[idx+7:]
		if endIdx := strings.Index(rawText, "```"); endIdx != -1 {
			rawText = rawText[:endIdx]
		}
		rawText = strings.TrimSpace(rawText)
	} else if idx := strings.Index(rawText, "```"); idx != -1 {
		rawText = rawText[idx+3:]
		if endIdx := strings.Index(rawText, "```"); endIdx != -1 {
			rawText = rawText[:endIdx]
		}
		rawText = strings.TrimSpace(rawText)
	}

	var brief repairBrief
	if err := json.Unmarshal([]byte(rawText), &brief); err != nil {
		return nil, fmt.Errorf("parse repair brief JSON: %w (raw: %s)", err, rawText)
	}

	return &brief, nil
}

func formatRepairBrief(b *repairBrief) string {
	var sb strings.Builder
	sb.WriteString("REPAIR BRIEF:\n")
	sb.WriteString(fmt.Sprintf("  CAUSE: %s\n", b.Cause))
	sb.WriteString(fmt.Sprintf("  KEEP: %s\n", b.Keep))
	sb.WriteString(fmt.Sprintf("  CHANGE: %s\n", b.Change))
	sb.WriteString(fmt.Sprintf("  DO NOT CHANGE: %s\n", b.DoNotChange))
	sb.WriteString(fmt.Sprintf("  NEXT ACTION: %s\n", b.NextAction))
	return sb.String()
}

func summarizeAcceptance(output string) string {
	if output == "" {
		return "no-output"
	}
	lines := strings.Split(output, "\n")
	var summary []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ok") || strings.HasPrefix(line, "FAIL") || strings.Contains(line, "PASS") {
			summary = append(summary, line)
		}
	}
	if len(summary) == 0 {
		if len(lines) > 5 {
			return strings.TrimSpace(strings.Join(lines[len(lines)-5:], "\n"))
		}
		return strings.TrimSpace(output)
	}
	return strings.TrimSpace(strings.Join(summary, "\n"))
}

func checkProgress(history []string, current string) bool {
	if len(history) == 0 {
		return true
	}
	last := history[len(history)-1]
	currentSum := summarizeAcceptance(current)

	if last == currentSum {
		return false
	}

	failCount := strings.Count(last, "FAIL")
	currentFailCount := strings.Count(currentSum, "FAIL")

	// More failures is not progress
	if currentFailCount > failCount {
		return false
	}

	// Fewer failures is progress
	if currentFailCount < failCount {
		return true
	}

	// Same failure count: pass after fail is progress
	if strings.Contains(currentSum, "PASS") && !strings.Contains(last, "PASS") {
		return true
	}

	// Same failure count but different failure messages: progress
	if currentSum != last {
		return true
	}

	return false
}

func buildCloserTask(job *jobs.Job, lastPatch, lastAcceptance, lastFailure string, progress []string) string {
	var sb strings.Builder
	sb.WriteString("## CLOSER TASK\n\n")
	sb.WriteString("Complete this bounded implementation job. Previous attempts failed.\n\n")
	sb.WriteString(fmt.Sprintf("### Original Task\n%s\n\n", job.Task))
	sb.WriteString(fmt.Sprintf("### Acceptance\n%s\n\n", strings.Join(job.Acceptance, " && ")))
	sb.WriteString(fmt.Sprintf("### Attempt History\n%s\n\n", strings.Join(progress, "\n---\n")))
	if lastPatch != "" {
		sb.WriteString(fmt.Sprintf("### Last Patch\n%s\n\n", lastPatch))
	}
	if lastAcceptance != "" {
		sb.WriteString(fmt.Sprintf("### Last Acceptance Output\n%s\n\n", lastAcceptance))
	}
	sb.WriteString(fmt.Sprintf("### Last Failure\n%s\n\n", lastFailure))
	sb.WriteString("Fix the code. Make acceptance pass. Do not redesign unrelated code.")
	return sb.String()
}

func runCloser(cfg *config.Config, closerTask string, tmpDir string, job *jobs.Job, garyBranch, acceptanceStr, gcsPath string) (bool, error) {
	fmt.Println("  Submitting Closer job...")

	jobName := fmt.Sprintf("gary-closer-%s-%s", job.ID, time.Now().Format("20060102-150405"))
	attemptGCSPath := fmt.Sprintf("%s/closer", gcsPath)

	codingCfg := batch.CodingJobConfig{
		Project:      cfg.GCPProject,
		Region:       cfg.GCPRegion,
		JobName:      jobName,
		RepoURL:      job.Repo,
		BaseBranch:   job.BaseBranch,
		GaryBranch:   garyBranch,
		Task:         closerTask,
		Acceptance:   acceptanceStr,
		GCSPath:      attemptGCSPath,
		MachineType:  "e2-standard-4",
		Model:        cfg.CloserModel,
		MaxRunDurSec: 900,
	}

	_, err := batch.SubmitCodingJob(codingCfg)
	if err != nil {
		return false, fmt.Errorf("submit closer: %w", err)
	}

	fmt.Println("  SUBMITTED")

	status, _, err := batch.WaitForTerminal(
		cfg.GCPProject, cfg.GCPRegion, jobName,
		10*time.Second, 20*time.Minute,
	)
	if err != nil {
		return false, fmt.Errorf("poll closer: %w", err)
	}

	if status.State() != "SUCCEEDED" {
		return false, fmt.Errorf("closer job %s", status.State())
	}

	patchData := downloadGCSFile(tmpDir, attemptGCSPath+"/result.patch")
	if patchData == "" {
		return false, fmt.Errorf("no patch from closer")
	}

	return runLocalAcceptance(tmpDir, job, garyBranch, patchData), nil
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

func saveResult(cfg *config.Config, job *jobs.Job, batchJobID, branch, commitSHA string, accepted bool, duration time.Duration, patchBytes, attempts, repairCount int, escalated, closerUsed bool, finalStatus string) {
	result := jobs.RunResult{
		JobID:        job.ID,
		BatchJobID:   batchJobID,
		Branch:       branch,
		CommitSHA:    commitSHA,
		Accepted:     accepted,
		DurationSecs: duration.Seconds(),
		PatchBytes:   int64(patchBytes),
		Attempts:     attempts,
		RepairCount:  repairCount,
		Escalated:    escalated,
		CloserUsed:   closerUsed,
		FinalStatus:  finalStatus,
	}

	dir := filepath.Join(cfg.ResultsDir, "runs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}

	filename := fmt.Sprintf("%s.json", job.ID)
	path := filepath.Join(dir, filename)

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(path, data, 0o644)
}
