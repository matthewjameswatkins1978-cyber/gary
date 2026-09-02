package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/matthewjameswatkins1978-cyber/gary/control"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/batch"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/config"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/cost"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/githubapp"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/gitops"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/jobs"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/project"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/supervisor"
)

const cloudCommandTimeout = 45 * time.Second

const (
	maxAttempts                 = 8
	maxLowProgressAttempts      = 3
	defaultWorkerMaxRunSec      = 3600
	workerSupervisionInterval   = 5 * time.Minute
	workerSupervisionPollPeriod = 10 * time.Second
	// Batch maxRunDuration covers task execution, not queueing/provisioning.
	// Keep enough bounded grace for a cold VM without abandoning an active job.
	batchSchedulingGraceSec = 900
)

var outputMu sync.Mutex

var batchGitHubTokenSource = githubapp.TokenFromEnvForRepository

func currentBatchGitHubToken(repoURL string) (string, error) {
	return batchGitHubTokenSource(repoURL)
}

func logPrint(prefix, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	outputMu.Lock()
	defer outputMu.Unlock()
	if prefix != "" {
		fmt.Printf("[%s] %s\n", prefix, strings.TrimRight(msg, "\n"))
	} else {
		fmt.Println(strings.TrimRight(msg, "\n"))
	}
}

func logPrintln(prefix, msg string) {
	outputMu.Lock()
	defer outputMu.Unlock()
	if prefix != "" {
		fmt.Printf("[%s] %s\n", prefix, msg)
	} else {
		fmt.Println(msg)
	}
}

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

	fmt.Println("GARY // RUN")
	fmt.Println()

	result := RunJob(cfg, job, "")

	fmt.Println()
	if result.FinalStatus == "DONE" {
		fmt.Println("ACCEPTED")
		fmt.Printf("Branch pushed: %s\n", result.Branch)
		fmt.Printf("Commit: %s\n", result.CommitSHA)
		fmt.Printf("Attempts: %d\n", result.Attempts)
		fmt.Printf("Duration: %.0fs\n", result.DurationSecs)
	} else {
		fmt.Printf("final_status=%s\n", result.FinalStatus)
	}
	if result.Cost != nil {
		fmt.Printf("Cost: $%.4f USD (%s)\n", result.Cost.TotalCostUSD, result.Cost.PricingStatus)
	}
}

// RunJob executes a single coding job with retry logic using the normal implementer model.
func RunJob(cfg *config.Config, job *jobs.Job, prefix string) jobs.RunResult {
	return runJobWithModelAttempts(cfg, job, prefix, cfg.ImplementerModel, maxAttempts, true, defaultWorkerMaxRunSec)
}

// RunJobWithModel executes a coding job with the normal repair loop and an explicit model.
func RunJobWithModel(cfg *config.Config, job *jobs.Job, prefix, model string) jobs.RunResult {
	return runJobWithModelAttempts(cfg, job, prefix, model, maxAttempts, true, defaultWorkerMaxRunSec)
}

// RunJobOnceWithModel executes exactly one Batch worker attempt.
// Yardmaster planning no longer uses Batch; this remains for bounded diagnostics.
func RunJobOnceWithModel(cfg *config.Config, job *jobs.Job, prefix, model string) jobs.RunResult {
	return runJobWithModelAttempts(cfg, job, prefix, model, 1, false, defaultWorkerMaxRunSec)
}

func runJobWithModelAttempts(cfg *config.Config, job *jobs.Job, prefix, model string, attemptLimit int, allowCloser bool, maxRunDurSec int) jobs.RunResult {
	if job.ProjectID != "" {
		ctx, err := project.DefaultStore().LoadContext(job.ProjectID)
		if err != nil {
			logPrint(prefix, "  PROJECT CONTEXT BLOCKED: %v", err)
			result := jobs.RunResult{JobID: job.ID, ProjectID: job.ProjectID, FinalStatus: "BLOCKED", FailureReason: err.Error(), Cost: cost.NewSummary()}
			saveRunResult(cfg, result)
			return result
		}
		if job.MasterPacketDigest != "" || job.ProjectManualVersion != 0 {
			if err := ctx.ValidateProvenance(job.ProjectID, job.MasterPacketDigest, job.ProjectManualVersion); err != nil {
				logPrint(prefix, "  PROJECT CONTEXT BLOCKED: %v", err)
				result := jobs.RunResult{JobID: job.ID, ProjectID: job.ProjectID, MasterPacketDigest: ctx.Project.MasterPacketDigest, ProjectManualVersion: ctx.Project.ProjectManualVersion, FinalStatus: "BLOCKED", FailureReason: err.Error(), Cost: cost.NewSummary()}
				saveRunResult(cfg, result)
				return result
			}
		}
		// Rebuild from the verified current manual; never trust a serialized
		// child context supplied by an older caller or worker.
		job.ProjectContext = ctx.ChildTaskForID(job.Task, job.ID)
		job.MasterPacketDigest = ctx.Project.MasterPacketDigest
		job.ProjectManualVersion = ctx.Project.ProjectManualVersion
	}
	if model == "" {
		model = cfg.ImplementerModel
	}

	suffix := randomSuffix(8)
	garyBranch := fmt.Sprintf("gary/%s-%s", job.ID, suffix)
	runID := fmt.Sprintf("%s-%s", job.ID, suffix)
	gcsPath := fmt.Sprintf("gs://gary-batch-results-418146339023/runs/%s", runID)
	acceptanceStr := strings.Join(job.Acceptance, " && ")

	logPrintln(prefix, fmt.Sprintf("Job        %s", job.ID))
	logPrintln(prefix, "Worker     batch")
	logPrintln(prefix, fmt.Sprintf("Branch     %s", garyBranch))

	submittedAt := time.Now()
	startedAt := submittedAt.UTC().Format(time.RFC3339)

	// Update snapshot: job is now active
	snapshotActiveJob(job.ID, "", job.Repo, job.BaseBranch, garyBranch, control.JobRunning, 0, startedAt, projectProvenance{job.ProjectID, job.MasterPacketDigest, job.ProjectManualVersion})

	// Budget check before starting
	if cfg.MaxCostUSD > 0 {
		estimated := EstimateJobCostForModel(model, 1)
		allowed, reason := BudgetCheck(cfg.MaxCostUSD, 0, estimated)
		if !allowed {
			logPrintln(prefix, "  BUDGET_STOPPED: "+reason)
			duration := time.Since(submittedAt)
			result := jobs.RunResult{
				JobID:              job.ID,
				Branch:             garyBranch,
				FinalStatus:        "BUDGET_STOPPED",
				FailureReason:      reason,
				DurationSecs:       duration.Seconds(),
				Attempts:           0,
				TotalCostEstimated: 0,
				Cost:               cost.NewSummary(),
			}
			saveRunResult(cfg, result)
			snapshotJobComplete(job.ID, result)
			return result
		}
	}

	var lastPatch string
	var lastAcceptance string
	var lastFailure string
	var lastRepairBrief string
	var lastProgressState string
	var prevGCSPatch string
	lastBatchJobID := ""
	lastBatchState := ""
	progressHistory := make([]string, 0)
	lowProgressStreak := 0
	repairCount := 0
	escalated := false
	closerUsed := false
	finalStatus := "FAILED"
	actualAttempts := 0
	runCost := cost.NewSummary()

attemptLoop:
	for attempt := 1; attempt <= attemptLimit; attempt++ {
		if cfg.MaxCostUSD > 0 {
			projected := EstimateJobCostForModel(model, 1)
			allowed, reason := BudgetCheck(cfg.MaxCostUSD, runCost.TotalCostUSD, projected)
			if !allowed {
				logPrintln(prefix, "  BUDGET_STOPPED: "+reason)
				finalStatus = "BUDGET_STOPPED"
				lastFailure = reason
				break
			}
		}
		actualAttempts = attempt

		attemptTask := job.Task
		if attempt > 1 {
			attemptTask = buildRetryTask(job.Task, acceptanceStr, lastProgressState, lastFailure, lastRepairBrief)
		}
		if job.ProjectContext != "" {
			attemptTask += "\n\n" + job.ProjectContext
		}

		jobName := fmt.Sprintf("gary-run-%s-%d-%s", job.ID, attempt, time.Now().Format("20060102-150405"))
		attemptGCSPath := fmt.Sprintf("%s/attempt-%d", gcsPath, attempt)

		attemptDir, err := os.MkdirTemp("", fmt.Sprintf("gary-%s-attempt-%d-*", job.ID, attempt))
		if err != nil {
			logPrint(prefix, "  create attempt dir: %v", err)
			finalStatus = "FAILED"
			break
		}

		logPrint(prefix, "ATTEMPT %d", attempt)

		// Update snapshot: retrying
		if attempt > 1 {
			snapshotActiveJob(job.ID, jobName, job.Repo, job.BaseBranch, garyBranch, control.JobRetrying, attempt, startedAt, projectProvenance{job.ProjectID, job.MasterPacketDigest, job.ProjectManualVersion})
		} else {
			snapshotActiveJob(job.ID, jobName, job.Repo, job.BaseBranch, garyBranch, control.JobRunning, attempt, startedAt, projectProvenance{job.ProjectID, job.MasterPacketDigest, job.ProjectManualVersion})
		}

		githubToken, tokenErr := currentBatchGitHubToken(job.Repo)
		if tokenErr != nil {
			logPrint(prefix, "  GitHub authentication refresh failed before Batch submission: %v", tokenErr)
			lastFailure = fmt.Sprintf("GitHub authentication refresh failed before Batch submission: %v", tokenErr)
			os.RemoveAll(attemptDir)
			actualAttempts = attempt - 1
			finalStatus = "BLOCKED"
			break
		}

		codingCfg := batch.CodingJobConfig{
			Project:          cfg.GCPProject,
			Region:           cfg.GCPRegion,
			JobName:          jobName,
			RepoURL:          job.Repo,
			BaseBranch:       job.BaseBranch,
			GaryBranch:       garyBranch,
			Task:             attemptTask,
			Acceptance:       acceptanceStr,
			EstimatedMinutes: job.EstimatedMinutes,
			GCSPath:          attemptGCSPath,
			PrevGCSPath:      prevGCSPatch,
			MachineType:      "e2-small",
			Model:            model,
			GitHubToken:      githubToken,
			MaxRunDurSec:     maxRunDurSec,
		}

		batchJobID, submitErr := batch.SubmitCodingJob(codingCfg)
		lastBatchJobID = batchJobID
		err = submitErr
		if err != nil {
			logPrint(prefix, "  submission failed: %v", err)
			lastFailure = fmt.Sprintf("Batch submission failed for %s: %v", jobName, err)
			os.RemoveAll(attemptDir)
			actualAttempts = attempt - 1
			finalStatus = "BLOCKED"
			break
		}

		logPrint(prefix, "  SUBMITTED %s", batchJobID)

		status, finishedAt, supervisionCosts, err := waitForSupervisedWorker(
			cfg, job, attemptTask, acceptanceStr, jobName, attemptGCSPath, prefix, maxRunDurSec,
		)
		for _, component := range supervisionCosts {
			runCost.Add(component)
		}
		if err != nil {
			logPrint(prefix, "  polling failed: %v", err)
			lastFailure = fmt.Sprintf("Batch polling failed for %s: %v", jobName, err)
			runCost.Add(batchCostComponent(model, "", time.Since(submittedAt).Seconds(), "e2-small"))
			// The worker may have uploaded its artifacts just before the
			// controller's deadline. Preserve them even though the terminal
			// Batch state was not observed.
			if recovered := downloadGCSFile(attemptDir, attemptGCSPath+"/result.patch"); recovered != "" {
				lastPatch = recovered
				prevGCSPatch = attemptGCSPath + "/result.patch"
				lastFailure += fmt.Sprintf("; recovered partial patch (%d bytes)", len(recovered))
			}
			if recovered := downloadGCSFile(attemptDir, attemptGCSPath+"/acceptance_output.txt"); recovered != "" {
				lastAcceptance = recovered
			}
			// Stop this exact job so a later wave cannot overlap an abandoned
			// worker. Its Batch ID remains in the durable result for recovery.
			if current, describeErr := batch.DescribeJob(cfg.GCPProject, cfg.GCPRegion, jobName); describeErr == nil && !batch.IsTerminal(current.State()) {
				if cancelErr := batch.CancelJob(cfg.GCPProject, cfg.GCPRegion, jobName); cancelErr != nil {
					lastFailure += "; cancel failed: " + cancelErr.Error()
				}
			}
			os.RemoveAll(attemptDir)
			finalStatus = "BLOCKED"
			break
		}
		attemptRuntime := batch.ObservedRuntimeSeconds(status, submittedAt, finishedAt)
		usageText := downloadGCSFile(attemptDir, attemptGCSPath+"/usage.jsonl")
		if usageText == "" {
			usageText = extractFromBatchLogs(cfg.GCPProject, cfg.GCPRegion, jobName, "GARY_USAGE_B64")
		}
		attemptCost := workerCostComponent(model, usageText, attemptRuntime)
		runCost.Add(attemptCost)

		batchState := status.State()
		lastBatchState = batchState
		batchSucceeded := batchState == "SUCCEEDED"

		patchData := downloadGCSFile(attemptDir, attemptGCSPath+"/result.patch")
		acceptOutput := downloadGCSFile(attemptDir, attemptGCSPath+"/acceptance_output.txt")

		// If GCS download failed, recover from Batch logs where possible.
		if patchData == "" {
			patchData = extractFromBatchLogs(cfg.GCPProject, cfg.GCPRegion, jobName, "GARY_PATCH_B64")
		}
		if acceptOutput == "" {
			acceptOutput = extractFromBatchLogs(cfg.GCPProject, cfg.GCPRegion, jobName, "GARY_ACCEPTANCE_B64")
		}

		if patchData != "" {
			lastPatch = patchData
			prevGCSPatch = attemptGCSPath + "/result.patch"
		}
		if acceptOutput != "" {
			lastAcceptance = acceptOutput
		}

		if strings.Contains(lastAcceptance, "SUPERVISION_TERMINAL=STOP_AND_REPLAN") {
			lastFailure = fmt.Sprintf("Yardmaster stopped worker for replan; partial patch/evidence preserved at %s", attemptGCSPath)
			finalStatus = "REPLAN_REQUIRED"
			logPrint(prefix, "  %s", lastFailure)
			os.RemoveAll(attemptDir)
			break
		}

		// Handover invariant: Batch state is transport evidence, not product
		// acceptance. Whenever a recoverable candidate exists, the trusted
		// control layer reruns the child's acceptance in a fresh clone before
		// deciding whether the work passed, failed, or needs repair.
		candidateAvailable := strings.TrimSpace(patchData) != ""
		if candidateAvailable {
			if batchSucceeded {
				logPrintln(prefix, "  BATCH SUCCEEDED — independently verifying candidate")
			} else {
				logPrint(prefix, "  BATCH %s BUT CANDIDATE EXISTS — independently verifying candidate", batchState)
			}

			localResult := runLocalAcceptanceClassified(attemptDir, job, garyBranch, patchData, prefix)
			if localResult.blocker {
				finalStatus = "BLOCKED"
				lastFailure = localResult.err.Error()
				logPrint(prefix, "  TRUSTED VERIFICATION BLOCKED: %s", lastFailure)
				break
			}
			if localResult.passed {
				workDir := filepath.Join(attemptDir, "repo")
				quality, qualityCost, qualityErr := runChildQualityInspection(cfg, job, workDir)
				runCost.Add(qualityCost)
				if qualityErr != nil {
					// A mechanically green candidate must never become accepted
					// merely because the quality gate is unavailable. Preserve
					// the exact candidate for recovery and stop at the control
					// boundary rather than throwing the work away.
					commitSHA, _ := gitops.CommitSHA(workDir)
					if pushErr := gitops.Push(workDir, "origin", garyBranch); pushErr != nil {
						lastFailure = fmt.Sprintf("Yardmaster quality inspection unavailable (%v); checkpoint push also failed: %v", qualityErr, pushErr)
						finalStatus = "BLOCKED"
						logPrint(prefix, "  QUALITY GATE BLOCKED: %s", lastFailure)
						os.RemoveAll(attemptDir)
						break attemptLoop
					}
					duration := time.Since(submittedAt)
					result := jobs.RunResult{
						JobID:                job.ID,
						ProjectID:            job.ProjectID,
						MasterPacketDigest:   job.MasterPacketDigest,
						ProjectManualVersion: job.ProjectManualVersion,
						BatchJobID:           jobName,
						BatchState:           batchState,
						ArtifactPrefix:       attemptGCSPath,
						Branch:               garyBranch,
						CommitSHA:            commitSHA,
						Accepted:             false,
						Verification:         "trusted-independent",
						QualityReview:        "UNAVAILABLE",
						DurationSecs:         duration.Seconds(),
						PatchBytes:           int64(len(patchData)),
						Attempts:             attempt,
						RepairCount:          repairCount,
						Escalated:            escalated,
						CloserUsed:           closerUsed,
						FinalStatus:          "BLOCKED",
						FailureReason:        compactFailureLogs("Yardmaster quality inspection unavailable: "+qualityErr.Error(), 4000),
						ModelCostEstimated:   runCost.ModelCostUSD,
						ComputeCostEstimated: runCost.ComputeCostUSD,
						TotalCostEstimated:   runCost.TotalCostUSD,
						Cost:                 runCost,
					}
					os.RemoveAll(attemptDir)
					saveRunResult(cfg, result)
					writeLedger(cfg, result, model)
					snapshotJobComplete(job.ID, result)
					return result
				}

				disposition := classifyChildQuality(quality)
				switch disposition {
				case childQualityPass:
					commitSHA, _ := gitops.CommitSHA(workDir)
					if err := gitops.Push(workDir, "origin", garyBranch); err != nil {
						lastFailure = fmt.Sprintf("GitHub writeback push failed for %s: %v", garyBranch, err)
						finalStatus = "BLOCKED"
						logPrint(prefix, "  SERVICE/ENVIRONMENT BLOCKED: %s", lastFailure)
						os.RemoveAll(attemptDir)
						break
					}
					duration := time.Since(submittedAt)
					os.RemoveAll(attemptDir)

					result := jobs.RunResult{
						JobID:                job.ID,
						ProjectID:            job.ProjectID,
						MasterPacketDigest:   job.MasterPacketDigest,
						ProjectManualVersion: job.ProjectManualVersion,
						BatchJobID:           jobName,
						BatchState:           batchState,
						ArtifactPrefix:       attemptGCSPath,
						Branch:               garyBranch,
						CommitSHA:            commitSHA,
						Accepted:             true,
						Verification:         "trusted-independent",
						QualityReview:        "PASS: " + strings.TrimSpace(quality.Summary),
						DurationSecs:         duration.Seconds(),
						PatchBytes:           int64(len(patchData)),
						Attempts:             attempt,
						RepairCount:          repairCount,
						Escalated:            escalated,
						CloserUsed:           closerUsed,
						FinalStatus:          "DONE",
						ModelCostEstimated:   runCost.ModelCostUSD,
						ComputeCostEstimated: runCost.ComputeCostUSD,
						TotalCostEstimated:   runCost.TotalCostUSD,
						Cost:                 runCost,
					}
					saveRunResult(cfg, result)
					writeLedger(cfg, result, model)
					snapshotJobComplete(job.ID, result)
					return result

				case childQualitySmallFix, childQualityMajorFix:
					repairCount++
					lastProgressState = "mechanically-passed-quality-repair"
					lastFailure = strings.TrimSpace(quality.Summary)
					lastRepairBrief = strings.TrimSpace(quality.Memory)
					progressHistory = append(progressHistory, "QUALITY_REVIEW "+lastFailure)
					if disposition == childQualitySmallFix {
						logPrint(prefix, "  YARDMASTER SMALL FIX: %s", lastFailure)
					} else {
						logPrint(prefix, "  YARDMASTER MAJOR FIX: %s", lastFailure)
					}
					if lastRepairBrief != "" {
						logPrint(prefix, "  YARDMASTER REPAIR GUIDANCE: %s", lastRepairBrief)
					}
					os.RemoveAll(attemptDir)
					if attempt == attemptLimit {
						escalated = true
						break attemptLoop
					}
					// The next worker attempt receives the checkpointed patch and
					// the Yardmaster's review. Small issues preserve the approach;
					// major issues receive stronger guidance while preserving any
					// useful work the review explicitly says to keep.
					continue attemptLoop

				case childQualityReplan:
					commitSHA, _ := gitops.CommitSHA(workDir)
					if pushErr := gitops.Push(workDir, "origin", garyBranch); pushErr != nil {
						lastFailure = fmt.Sprintf("%s; checkpoint push failed: %v", strings.TrimSpace(quality.Summary), pushErr)
						finalStatus = "BLOCKED"
						os.RemoveAll(attemptDir)
						break attemptLoop
					}
					duration := time.Since(submittedAt)
					result := jobs.RunResult{
						JobID:                job.ID,
						ProjectID:            job.ProjectID,
						MasterPacketDigest:   job.MasterPacketDigest,
						ProjectManualVersion: job.ProjectManualVersion,
						BatchJobID:           jobName,
						BatchState:           batchState,
						ArtifactPrefix:       attemptGCSPath,
						Branch:               garyBranch,
						CommitSHA:            commitSHA,
						Accepted:             false,
						Verification:         "trusted-independent",
						QualityReview:        "REPLAN: " + strings.TrimSpace(quality.Summary),
						DurationSecs:         duration.Seconds(),
						PatchBytes:           int64(len(patchData)),
						Attempts:             attempt,
						RepairCount:          repairCount,
						Escalated:            escalated,
						CloserUsed:           closerUsed,
						FinalStatus:          "REPLAN_REQUIRED",
						FailureReason:        compactFailureLogs(strings.TrimSpace(quality.Summary)+"\n"+strings.TrimSpace(quality.Memory), 4000),
						ModelCostEstimated:   runCost.ModelCostUSD,
						ComputeCostEstimated: runCost.ComputeCostUSD,
						TotalCostEstimated:   runCost.TotalCostUSD,
						Cost:                 runCost,
					}
					os.RemoveAll(attemptDir)
					saveRunResult(cfg, result)
					writeLedger(cfg, result, model)
					snapshotJobComplete(job.ID, result)
					return result
				}
			}
			if !localResult.passed {
				lastFailure = fmt.Sprintf("trusted acceptance rejected recovered candidate after Batch state %s: %v", batchState, localResult.err)
			}
		}

		if !batchSucceeded {
			logPrint(prefix, "  BATCH %s", batchState)
			if lastFailure == "" {
				lastFailure = fmt.Sprintf("Batch job ended with state: %s\n", batchState)
			} else {
				lastFailure += fmt.Sprintf("\nBatch job ended with state: %s\n", batchState)
			}
			failureLogs := ""
			if status.UID != "" {
				if logs, logErr := batch.RecentLogs(cfg.GCPProject, status.UID, 12); logErr == nil {
					failureLogs = compactFailureLogs(logs, 4000)
					if failureLogs != "" {
						logPrint(prefix, "  Batch failure logs:\n%s", failureLogs)
						lastFailure += "\nBatch failure logs:\n" + failureLogs
					}
				}
			}
			if lastAcceptance != "" {
				lastFailure += "\nWorker acceptance output:\n" + lastAcceptance
			}
			if isServiceEnvironmentBlocker(failureLogs) {
				finalStatus = "BLOCKED"
				if actualAttempts > 0 {
					actualAttempts--
				}
				logPrint(prefix, "  SERVICE/ENVIRONMENT BLOCKED: %s", strings.TrimSpace(failureLogs))
				break
			}
		} else if !candidateAvailable {
			lastFailure = "worker completed without a recoverable result patch"
			logPrintln(prefix, "  BATCH SUCCEEDED but produced no patch — refusing empty writeback")
		}
		os.RemoveAll(attemptDir)

		progressState := "unknown"
		if acceptOutput != "" {
			improved := checkProgress(progressHistory, acceptOutput)
			if improved {
				lowProgressStreak = 0
				progressState = "improving"
				logPrintln(prefix, "  PROGRESS — acceptance evidence moved toward the target")
			} else {
				lowProgressStreak++
				progressState = "stalled"
				logPrint(prefix, "  LOW PROGRESS — stalled evidence (%d/%d)", lowProgressStreak, maxLowProgressAttempts)
			}
			progressHistory = append(progressHistory, summarizeAcceptance(acceptOutput))
		} else {
			progressState = "unknown-no-acceptance-evidence"
			logPrintln(prefix, "  NO ACCEPTANCE EVIDENCE — not counting this as low progress")
		}
		lastProgressState = progressState

		if (acceptOutput != "" || patchData != "") && cfg.GeminiAPIKey != "" && cfg.CloserModel != "" {
			patchForReview := patchData
			if patchForReview == "" {
				patchForReview = lastPatch
			}
			brief, repairCost, briefErr := generateRepairBrief(
				cfg,
				job.Task,
				acceptanceStr,
				patchForReview,
				acceptOutput,
				lastFailure,
				progressState,
				lastRepairBrief,
			)
			runCost.Add(repairCost)
			if briefErr != nil {
				logPrint(prefix, "  repair review unavailable: %v", briefErr)
			} else {
				lastRepairBrief = formatRepairBrief(brief)
				repairCount++
				logPrint(prefix, "  STRONGER-AGENT GUIDANCE\n%s", lastRepairBrief)
			}
		}

		if allowCloser && lowProgressStreak >= maxLowProgressAttempts {
			escalated = true
			logPrintln(prefix, "  STALLED — escalating after repeated low-progress attempts")
			break
		}
		if attempt == attemptLimit {
			if allowCloser {
				escalated = true
				logPrintln(prefix, "  SAFETY ATTEMPT LIMIT REACHED — handing accumulated evidence to Closer")
			}
			break
		}

		logPrintln(prefix, "  CONTINUING — previous attempt is evidence, not a job verdict")
	}

	if escalated && allowCloser {
		logPrintln(prefix, "ESCALATING TO CLOSER")
		closerDir, err := os.MkdirTemp("", fmt.Sprintf("gary-%s-closer-*", job.ID))
		if err != nil {
			logPrint(prefix, "  create closer dir: %v", err)
			finalStatus = "FAILED"
		} else {
			defer os.RemoveAll(closerDir)

			closerTask := buildCloserTask(job, lastPatch, lastAcceptance, lastFailure, progressHistory)
			closeResult, closerCost, err := runCloser(cfg, closerTask, closerDir, job, garyBranch, acceptanceStr, gcsPath, prevGCSPatch, prefix)
			runCost.Add(closerCost)
			if err != nil {
				logPrint(prefix, "  closer failed: %v", err)
				if isServiceEnvironmentBlocker(err.Error()) {
					finalStatus = "BLOCKED"
				} else {
					finalStatus = "FAILED"
				}
			} else if closeResult {
				workDir := filepath.Join(closerDir, "repo")
				quality, qualityCost, qualityErr := runChildQualityInspection(cfg, job, workDir)
				runCost.Add(qualityCost)
				if qualityErr != nil {
					finalStatus = "BLOCKED"
					lastFailure = "Closer candidate passed trusted mechanics but Yardmaster quality inspection was unavailable: " + qualityErr.Error()
					logPrint(prefix, "  QUALITY GATE BLOCKED: %s", lastFailure)
				} else if classifyChildQuality(quality) != childQualityPass {
					commitSHA, _ := gitops.CommitSHA(workDir)
					if pushErr := gitops.Push(workDir, "origin", garyBranch); pushErr != nil {
						finalStatus = "BLOCKED"
						lastFailure = fmt.Sprintf("Closer quality review %s; checkpoint push failed: %v", strings.TrimSpace(quality.Summary), pushErr)
					} else {
						duration := time.Since(submittedAt)
						result := jobs.RunResult{
							JobID:                job.ID,
							ProjectID:            job.ProjectID,
							MasterPacketDigest:   job.MasterPacketDigest,
							ProjectManualVersion: job.ProjectManualVersion,
							ArtifactPrefix:       gcsPath + "/closer",
							Branch:               garyBranch,
							CommitSHA:            commitSHA,
							Accepted:             false,
							Verification:         "trusted-independent",
							QualityReview:        strings.TrimSpace(quality.Summary),
							DurationSecs:         duration.Seconds(),
							Attempts:             actualAttempts,
							RepairCount:          repairCount,
							Escalated:            true,
							CloserUsed:           true,
							FinalStatus:          "REPLAN_REQUIRED",
							FailureReason:        compactFailureLogs(strings.TrimSpace(quality.Summary)+"\n"+strings.TrimSpace(quality.Memory), 4000),
							ModelCostEstimated:   runCost.ModelCostUSD,
							ComputeCostEstimated: runCost.ComputeCostUSD,
							TotalCostEstimated:   runCost.TotalCostUSD,
							Cost:                 runCost,
						}
						saveRunResult(cfg, result)
						writeLedger(cfg, result, model+"+"+cfg.CloserModel)
						snapshotJobComplete(job.ID, result)
						return result
					}
				} else {
					commitSHA, _ := gitops.CommitSHA(workDir)
					if err := gitops.Push(workDir, "origin", garyBranch); err != nil {
						finalStatus = "BLOCKED"
						lastFailure = fmt.Sprintf("GitHub writeback push failed for %s: %v", garyBranch, err)
						logPrint(prefix, "  SERVICE/ENVIRONMENT BLOCKED: %s", lastFailure)
					} else {
						duration := time.Since(submittedAt)
						closerUsed = true
						finalStatus = "DONE"

						result := jobs.RunResult{
							JobID:                job.ID,
							ProjectID:            job.ProjectID,
							MasterPacketDigest:   job.MasterPacketDigest,
							ProjectManualVersion: job.ProjectManualVersion,
							ArtifactPrefix:       gcsPath + "/closer",
							Branch:               garyBranch,
							CommitSHA:            commitSHA,
							Accepted:             true,
							Verification:         "trusted-independent",
							QualityReview:        "PASS: " + strings.TrimSpace(quality.Summary),
							DurationSecs:         duration.Seconds(),
							Attempts:             actualAttempts,
							RepairCount:          repairCount,
							Escalated:            escalated,
							CloserUsed:           closerUsed,
							FinalStatus:          "DONE",
							ModelCostEstimated:   runCost.ModelCostUSD,
							ComputeCostEstimated: runCost.ComputeCostUSD,
							TotalCostEstimated:   runCost.TotalCostUSD,
							Cost:                 runCost,
						}
						saveRunResult(cfg, result)
						writeLedger(cfg, result, model+"+"+cfg.CloserModel)
						snapshotJobComplete(job.ID, result)
						return result
					}
				}
			} else {
				finalStatus = "FAILED"
			}
		}
	}

	duration := time.Since(submittedAt)

	result := jobs.RunResult{
		JobID:                job.ID,
		ProjectID:            job.ProjectID,
		BatchJobID:           lastBatchJobID,
		BatchState:           lastBatchState,
		ArtifactPrefix:       gcsPath,
		MasterPacketDigest:   job.MasterPacketDigest,
		ProjectManualVersion: job.ProjectManualVersion,
		Branch:               garyBranch,
		PatchBytes:           int64(len(lastPatch)),
		DurationSecs:         duration.Seconds(),
		Attempts:             actualAttempts,
		RepairCount:          repairCount,
		Escalated:            escalated,
		CloserUsed:           closerUsed,
		FinalStatus:          finalStatus,
		FailureReason:        compactFailureLogs(lastFailure, 4000),
		ModelCostEstimated:   runCost.ModelCostUSD,
		ComputeCostEstimated: runCost.ComputeCostUSD,
		TotalCostEstimated:   runCost.TotalCostUSD,
		Cost:                 runCost,
	}
	saveRunResult(cfg, result)
	writeLedger(cfg, result, model)
	snapshotJobComplete(job.ID, result)
	return result
}
func waitForSupervisedWorker(cfg *config.Config, job *jobs.Job, task, acceptance, jobName, gcsPath, prefix string, maxRunDurSec int) (*batch.JobStatus, time.Time, []cost.Component, error) {
	deadline := time.Now().Add(time.Duration(maxRunDurSec+batchSchedulingGraceSec) * time.Second)
	lastState := ""
	lastReviewID := ""
	lastReviewAt := time.Time{}
	generation := 0
	components := make([]cost.Component, 0)

	for {
		if time.Now().After(deadline) {
			return nil, time.Time{}, components, fmt.Errorf("emergency infrastructure ceiling reached while waiting for supervised job %s", jobName)
		}

		status, err := batch.DescribeJob(cfg.GCPProject, cfg.GCPRegion, jobName)
		if err != nil {
			return nil, time.Time{}, components, fmt.Errorf("describe supervised job: %w", err)
		}
		if status.State() != lastState {
			logPrint(prefix, "  BATCH STATE %s", status.State())
			lastState = status.State()
		}
		if batch.IsTerminal(status.State()) {
			return status, time.Now(), components, nil
		}

		telemetry, ok := readWorkerTelemetry(gcsPath + "/telemetry.json")
		if ok {
			now := time.Now()
			due := false
			switch telemetry.Phase {
			case "candidate_ready", "turn_complete":
				due = telemetry.ReviewID != lastReviewID
			case "running":
				due = telemetry.ElapsedActiveSeconds >= int(workerSupervisionInterval.Seconds()) &&
					telemetry.ReviewID != lastReviewID &&
					(lastReviewAt.IsZero() || now.Sub(lastReviewAt) >= workerSupervisionInterval)
			}
			if due {
				generation++
				decision, component, superviseErr := runWorkerSupervision(
					cfg, task, acceptance, job.ProjectContext, telemetry, generation,
				)
				components = append(components, component)
				if superviseErr != nil {
					// Supervision failures must not kill productive work. Keep a
					// running worker alive; let a naturally completed turn resume,
					// and let a mechanically ready candidate proceed to inspection.
					decision = fallbackSupervisionDecision(telemetry, generation, superviseErr)
					logPrint(prefix, "  YARDMASTER SUPERVISION UNAVAILABLE: %v; fallback=%s", superviseErr, decision.Action)
				} else {
					logPrint(prefix, "  YARDMASTER %s: %s", decision.Action, decision.Reason)
					if decision.Action == supervisor.ActionGuide {
						logPrint(prefix, "  YARDMASTER GUIDANCE: %s", decision.Guidance)
					}
				}
				if err := writeWorkerControl(gcsPath+"/control.json", decision); err != nil {
					return nil, time.Time{}, components, fmt.Errorf("publish Yardmaster control: %w", err)
				}
				lastReviewID = telemetry.ReviewID
				lastReviewAt = now
			}
		}

		time.Sleep(workerSupervisionPollPeriod)
	}
}

func fallbackSupervisionDecision(telemetry supervisor.Telemetry, generation int, cause error) supervisor.Decision {
	action := supervisor.ActionContinue
	reason := "supervisor call unavailable; preserve productive work and continue"
	if telemetry.Phase == "candidate_ready" {
		action = supervisor.ActionReadyForInspection
		reason = "supervisor call unavailable after mechanical candidate readiness; hand candidate to independent/local inspection"
	}
	decision := supervisor.Decision{Action: action, Reason: reason + ": " + compactFailureLogs(cause.Error(), 500)}
	_ = decision.Normalize(generation, telemetry.ReviewID, time.Now())
	return decision
}

func readWorkerTelemetry(path string) (supervisor.Telemetry, bool) {
	data, err := readGCSObject(path)
	if err != nil || len(data) == 0 {
		return supervisor.Telemetry{}, false
	}
	var telemetry supervisor.Telemetry
	if err := json.Unmarshal(data, &telemetry); err != nil {
		return supervisor.Telemetry{}, false
	}
	if err := telemetry.Validate(); err != nil {
		return supervisor.Telemetry{}, false
	}
	return telemetry, true
}

func writeWorkerControl(path string, decision supervisor.Decision) error {
	data, err := json.Marshal(decision)
	if err != nil {
		return err
	}
	return writeGCSObject(path, data)
}

func readGCSObject(path string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cloudCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gcloud", "storage", "cat", path)
	cmd.WaitDelay = 2 * time.Second
	out, err := cmd.CombinedOutput()
	if err == nil {
		return out, nil
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), cloudCommandTimeout)
	defer cancel2()
	fallback := exec.CommandContext(ctx2, "python3", "-m", "gsutil", "cat", path)
	fallback.WaitDelay = 2 * time.Second
	out2, err2 := fallback.CombinedOutput()
	if err2 != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return out2, nil
}

func writeGCSObject(path string, data []byte) error {
	tmp, err := os.CreateTemp("", "gary-supervision-*.json")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cloudCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gcloud", "storage", "cp", name, path)
	cmd.WaitDelay = 2 * time.Second
	if out, err := cmd.CombinedOutput(); err == nil {
		return nil
	} else {
		ctx2, cancel2 := context.WithTimeout(context.Background(), cloudCommandTimeout)
		defer cancel2()
		fallback := exec.CommandContext(ctx2, "python3", "-m", "gsutil", "cp", name, path)
		fallback.WaitDelay = 2 * time.Second
		if out2, err2 := fallback.CombinedOutput(); err2 != nil {
			return fmt.Errorf("write %s: gcloud=%v (%s); gsutil=%v (%s)", path, err, strings.TrimSpace(string(out)), err2, strings.TrimSpace(string(out2)))
		}
		return nil
	}
}

func downloadGCSFile(tmpDir, gcsPath string) string {
	path := filepath.Join(tmpDir, filepath.Base(gcsPath))
	for _, args := range [][]string{
		{"gsutil", "cp", gcsPath, path},
		{"gcloud", "storage", "cp", gcsPath, path},
	} {
		ctx, cancel := context.WithTimeout(context.Background(), cloudCommandTimeout)
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.WaitDelay = 2 * time.Second
		if _, err := cmd.CombinedOutput(); err == nil {
			data, readErr := os.ReadFile(path)
			if readErr == nil {
				cancel()
				return string(data)
			}
		}
		cancel()
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

// extractFromBatchLogs retrieves base64-encoded data from Cloud Logging.
// It queries the exact Batch job's execution window, then filters structured
// entries locally because older Batch log entries do not support reliable
// server-side UID filtering. The marker query must not be applied server-side:
// the payload lines do not contain the marker text.
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
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Timestamp < entries[j].Timestamp
	})

	return extractMarkerFromBatchLogs(entries, status.UID, marker)
}

func extractMarkerFromBatchLogs(entries []batchLogEntry, jobUID, marker string) string {
	startMarker := marker + "_START"
	endMarker := marker + "_END"
	var b64Lines []string
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
			b64Lines = append(b64Lines, line)
		}
	}

	if len(b64Lines) == 0 {
		return ""
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.Join(b64Lines, ""))
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

func runLocalAcceptance(tmpDir string, job *jobs.Job, garyBranch, patchData, prefix string) bool {
	return runLocalAcceptanceClassified(tmpDir, job, garyBranch, patchData, prefix).passed
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
		logPrint(prefix, "    LOCAL ENVIRONMENT BLOCKED: %v", err)
		return fail(err, true)
	}
	defer cleanupPython()

	for _, command := range job.Acceptance {
		out, err := runAcceptanceCommand(workDir, command, pythonBin)
		if err != nil {
			logPrint(prefix, "    LOCAL FAIL: %s", strings.TrimSpace(string(out)))
			return fail(fmt.Errorf("local acceptance command %q failed: %w", command, err), false)
		}
		logPrintln(prefix, "    LOCAL PASS")
	}

	if err := cleanupDisposableWorkerArtifacts(workDir); err != nil {
		logPrint(prefix, "    LOCAL CLEANUP FAIL: %v", err)
		return fail(err, true)
	}
	if err := gitops.CommitAll(workDir, fmt.Sprintf("GARY: %s", job.Task)); err != nil {
		logPrint(prefix, "    LOCAL COMMIT FAIL: %v", err)
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

type repairBrief struct {
	Progress    string `json:"progress"`
	Evidence    string `json:"evidence"`
	Cause       string `json:"cause"`
	Keep        string `json:"keep"`
	Change      string `json:"change"`
	DoNotChange string `json:"do_not_change"`
	NextAction  string `json:"next_action"`
}

func generateRepairBrief(cfg *config.Config, task, acceptance, patch, acceptOutput, failureEvidence, progressState, priorBrief string) (*repairBrief, cost.Component, error) {
	prompt := fmt.Sprintf(`You are the stronger review agent guiding a smaller implementation worker toward mechanical acceptance.

Do not merely say that the attempt is wrong. Use the evidence to tell the next worker what already works, what remains wrong, what to preserve, and exactly what to try next.
An unsuccessful attempt that improves acceptance is progress, not a reason to discard useful work or change approach unnecessarily.

ORIGINAL TASK:
%s

ACCEPTANCE CRITERIA:
%s

CURRENT PROGRESS STATUS:
%s

CURRENT CUMULATIVE PATCH:
%s

ACCEPTANCE OUTPUT:
%s

FAILURE EVIDENCE:
%s

PRIOR REPAIR BRIEF:
%s

Respond with exactly this JSON format:
{
  "progress": "improving, stalled, regressed, or unknown",
  "evidence": "the specific mechanical evidence supporting that judgement",
  "cause": "most likely remaining cause in one sentence",
  "keep": "working partial changes that must be preserved",
  "change": "the smallest concrete change to try next",
  "do_not_change": "working behavior, scope, tests, or constraints that must remain unchanged",
  "next_action": "one direct instruction the smaller worker can execute next"
}`, task, acceptance, progressState, patch, acceptOutput, failureEvidence, priorBrief)

	return callGeminiJSON(cfg, prompt)
}

func callGeminiJSON(cfg *config.Config, prompt string) (*repairBrief, cost.Component, error) {
	component := estimatedComponent("repair", cfg.CloserModel, "repair response did not return provider usage")
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
		return nil, component, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, component, fmt.Errorf("gemini request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, component, fmt.Errorf("gemini request: %w", err)
	}
	defer resp.Body.Close()
	component.ProviderRequestID = resp.Header.Get("x-goog-request-id")

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, component, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		component.UnmeteredReason = fmt.Sprintf("repair HTTP %d returned no usable usage record", resp.StatusCode)
		return nil, component, fmt.Errorf("gemini API %d: %s", resp.StatusCode, string(respBody))
	}
	if usage, usageErr := cost.ParseVertexUsage(json.RawMessage(respBody), cfg.CloserModel, "", component.ProviderRequestID); usageErr == nil {
		component = componentFromUsage("repair", usage)
		component.ProviderRequestID = resp.Header.Get("x-goog-request-id")
	} else {
		component.UnmeteredReason = "repair response lacked complete usageMetadata: " + usageErr.Error()
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
		return nil, component, fmt.Errorf("parse response: %w", err)
	}

	if len(apiResp.Candidates) == 0 || len(apiResp.Candidates[0].Content.Parts) == 0 {
		return nil, component, fmt.Errorf("no candidates in response")
	}

	rawText := apiResp.Candidates[0].Content.Parts[0].Text
	rawText = strings.TrimSpace(rawText)

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
		return nil, component, fmt.Errorf("parse repair brief JSON: %w (raw: %s)", err, rawText)
	}

	return &brief, component, nil
}

func formatRepairBrief(b *repairBrief) string {
	var sb strings.Builder
	sb.WriteString("REPAIR BRIEF:\n")
	sb.WriteString(fmt.Sprintf("  PROGRESS: %s\n", b.Progress))
	sb.WriteString(fmt.Sprintf("  EVIDENCE: %s\n", b.Evidence))
	sb.WriteString(fmt.Sprintf("  CAUSE: %s\n", b.Cause))
	sb.WriteString(fmt.Sprintf("  KEEP: %s\n", b.Keep))
	sb.WriteString(fmt.Sprintf("  CHANGE: %s\n", b.Change))
	sb.WriteString(fmt.Sprintf("  DO NOT CHANGE: %s\n", b.DoNotChange))
	sb.WriteString(fmt.Sprintf("  NEXT ACTION: %s\n", b.NextAction))
	return sb.String()
}

func buildRetryTask(originalTask, acceptance, progressState, failureEvidence, repairGuidance string) string {
	var sb strings.Builder
	sb.WriteString(originalTask)
	sb.WriteString("\n\n## REPAIR ITERATION\n\n")
	sb.WriteString("The previous attempt did not yet satisfy acceptance. This is evidence, not a job verdict. Continue from the existing partial work; do not start over unless the evidence specifically requires it.\n\n")
	if progressState != "" {
		sb.WriteString(fmt.Sprintf("Progress status: %s\n\n", progressState))
	}
	sb.WriteString("### Acceptance still required\n")
	sb.WriteString(acceptance)
	sb.WriteString("\n\n")
	if repairGuidance != "" {
		sb.WriteString("### Stronger-agent repair guidance\n")
		sb.WriteString(repairGuidance)
		sb.WriteString("\n")
	}
	if failureEvidence != "" {
		sb.WriteString("### Raw failure evidence\n")
		sb.WriteString(failureEvidence)
		sb.WriteString("\n")
	}
	sb.WriteString("\nPreserve what already works. Make the smallest evidence-driven change that moves the stated acceptance closer to passing, then run the stated checks.")
	return sb.String()
}

func compactFailureLogs(logs string, maxBytes int) string {
	logs = strings.TrimSpace(logs)
	if maxBytes <= 0 || len(logs) <= maxBytes {
		return logs
	}
	return "[truncated]\n" + logs[len(logs)-maxBytes:]
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

	if currentFailCount > failCount {
		return false
	}
	if currentFailCount < failCount {
		return true
	}
	if strings.Contains(currentSum, "PASS") && !strings.Contains(last, "PASS") {
		return true
	}
	if currentSum != last {
		return true
	}

	return false
}

func buildCloserTask(job *jobs.Job, lastPatch, lastAcceptance, lastFailure string, progress []string) string {
	var sb strings.Builder
	sb.WriteString("## CLOSER TASK\n\n")
	sb.WriteString("Complete this bounded implementation job. Previous attempts did not yet satisfy acceptance. Preserve useful partial work and use the accumulated evidence.\n\n")
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

func runCloser(cfg *config.Config, closerTask string, tmpDir string, job *jobs.Job, garyBranch, acceptanceStr, gcsPath, prevGCSPatch, prefix string) (bool, cost.Component, error) {
	logPrintln(prefix, "  Submitting Closer job...")
	closerCost := batchCostComponent(cfg.CloserModel, "", 0, "e2-standard-4")
	startedAt := time.Now()

	jobName := fmt.Sprintf("gary-closer-%s-%s", job.ID, time.Now().Format("20060102-150405"))
	attemptGCSPath := fmt.Sprintf("%s/closer", gcsPath)

	githubToken, err := currentBatchGitHubToken(job.Repo)
	if err != nil {
		return false, closerCost, fmt.Errorf("GitHub authentication refresh before Closer submission: %w", err)
	}

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
		PrevGCSPath:  prevGCSPatch,
		MachineType:  "e2-standard-4",
		Model:        cfg.CloserModel,
		GitHubToken:  githubToken,
		MaxRunDurSec: 900,
	}

	_, err = batch.SubmitCodingJob(codingCfg)
	if err != nil {
		return false, closerCost, fmt.Errorf("submit closer: %w", err)
	}

	logPrint(prefix, "  SUBMITTED %s", jobName)

	status, finishedAt, err := batch.WaitForTerminal(
		cfg.GCPProject, cfg.GCPRegion, jobName,
		10*time.Second, 20*time.Minute,
	)
	if err != nil {
		closerCost.UnmeteredReason = appendCostReason(closerCost.UnmeteredReason, "closer Batch polling failed after submission")
		return false, closerCost, fmt.Errorf("poll closer: %w", err)
	}
	usageText := downloadGCSFile(tmpDir, attemptGCSPath+"/usage.jsonl")
	if usageText == "" {
		usageText = extractFromBatchLogs(cfg.GCPProject, cfg.GCPRegion, jobName, "GARY_USAGE_B64")
	}
	closerCost = batchCostComponent(cfg.CloserModel, usageText, batch.ObservedRuntimeSeconds(status, startedAt, finishedAt), "e2-standard-4")
	closerCost.Kind = "closer"

	patchData := downloadGCSFile(tmpDir, attemptGCSPath+"/result.patch")
	if patchData == "" {
		patchData = extractFromBatchLogs(cfg.GCPProject, cfg.GCPRegion, jobName, "GARY_PATCH_B64")
	}
	if patchData == "" {
		if status.State() != "SUCCEEDED" {
			closerCost.UnmeteredReason = appendCostReason(closerCost.UnmeteredReason, "closer Batch job ended in "+status.State()+" with no recoverable candidate")
			return false, closerCost, fmt.Errorf("closer job %s with no recoverable candidate", status.State())
		}
		return false, closerCost, fmt.Errorf("no patch from closer")
	}
	if status.State() != "SUCCEEDED" {
		closerCost.UnmeteredReason = appendCostReason(closerCost.UnmeteredReason, "closer Batch job ended in "+status.State()+" but a candidate was recovered and independently verified")
		logPrint(prefix, "  CLOSER BATCH %s BUT CANDIDATE EXISTS — independently verifying candidate", status.State())
	}

	return runLocalAcceptance(tmpDir, job, garyBranch, patchData, prefix), closerCost, nil
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

func saveRunResult(cfg *config.Config, result jobs.RunResult) {
	dir := filepath.Join(cfg.ResultsDir, "runs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}

	filename := fmt.Sprintf("%s.json", result.JobID)
	path := filepath.Join(dir, filename)

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(path, data, 0o644)
}

func batchCostComponent(model, usageText string, runtimeSeconds float64, machineType string) cost.Component {
	var component cost.Component
	if usage, err := cost.ParseGeminiCLIStreamJSONL([]byte(usageText), model); err == nil {
		component = componentFromUsage("worker", usage)
	} else {
		component = estimatedComponent("worker", model, "Gemini CLI provider telemetry unavailable: "+err.Error())
	}
	component.RuntimeSeconds = runtimeSeconds
	component.MachineType = machineType
	component.ProvisioningModel = "STANDARD"
	computeCost, _ := cost.ComputeCost(component.MachineType, component.ProvisioningModel, runtimeSeconds)
	component.ComputeCostUSD = computeCost
	component.TotalCostUSD = component.ModelCostUSD + component.ComputeCostUSD
	component.UnmeteredReason = appendCostReason(component.UnmeteredReason, "Batch compute is estimated from observed runtime; invoice data was not queried")
	return component
}

func workerCostComponent(model, usageText string, runtimeSeconds float64) cost.Component {
	return batchCostComponent(model, usageText, runtimeSeconds, "e2-small")
}

func appendCostReason(existing, addition string) string {
	if existing == "" {
		return addition
	}
	return existing + "; " + addition
}

func writeLedger(cfg *config.Config, result jobs.RunResult, model string) {
	entry := MakeLedgerEntry(result, model)
	if err := AppendLedger(cfg.ResultsDir, entry); err != nil {
		logPrint("", "  ledger write failed: %v", err)
	}
}
