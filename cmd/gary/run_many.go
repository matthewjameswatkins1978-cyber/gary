package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/config"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/cost"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/jobs"
)

func cmdRunMany() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: gary run-many <job-file> ...")
		os.Exit(1)
	}

	jobFiles := os.Args[2:]

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

	maxWorkers := 4
	if v := os.Getenv("GARY_MAX_WORKERS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			fmt.Fprintf(os.Stderr, "invalid GARY_MAX_WORKERS: %s (must be >= 1)\n", v)
			os.Exit(1)
		}
		maxWorkers = n
	}

	type jobEntry struct {
		path string
		job  *jobs.Job
	}
	entries := make([]jobEntry, 0, len(jobFiles))
	for _, path := range jobFiles {
		job, err := loadJob(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load job %s: %v\n", path, err)
			os.Exit(1)
		}
		if job.ID == "" || job.Repo == "" || job.BaseBranch == "" || job.Task == "" {
			fmt.Fprintf(os.Stderr, "job %s requires id, repo, base_branch, and task\n", path)
			os.Exit(1)
		}
		if len(job.Acceptance) == 0 {
			job.Acceptance = []string{"echo no-acceptance-command"}
		}
		entries = append(entries, jobEntry{path: path, job: job})
	}

	fmt.Println("GARY // RUN-MANY")
	fmt.Printf("Jobs:      %d\n", len(entries))
	fmt.Printf("Workers:   %d\n", maxWorkers)
	if cfg.MaxCostUSD > 0 {
		fmt.Printf("Max cost:  $%.2f ESTIMATED\n", cfg.MaxCostUSD)
	}
	fmt.Println()

	// Run concurrently with per-run budget tracking
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup
	results := make([]jobs.RunResult, len(entries))
	var runCommitted atomic.Int64 // fixed-point * 10000
	var runSpend atomic.Int64     // fixed-point * 10000

	for i, entry := range entries {
		wg.Add(1)
		go func(idx int, e jobEntry) {
			defer wg.Done()

			// Budget check: reserve cost for this job
			if cfg.MaxCostUSD > 0 {
				jobCost := EstimateJobCost(1)
				for {
					old := runCommitted.Load()
					committed := float64(old) / 10000
					allowed, reason := BudgetCheck(cfg.MaxCostUSD, committed, jobCost)
					if !allowed {
						logPrint(e.job.ID, "BUDGET_STOPPED: %s", reason)
						results[idx] = jobs.RunResult{
							JobID:       e.job.ID,
							FinalStatus: "BUDGET_STOPPED",
						}
						return
					}
					newCommitted := int64((committed + jobCost) * 10000)
					if runCommitted.CompareAndSwap(old, newCommitted) {
						break
					}
				}
			}

			sem <- struct{}{}
			defer func() { <-sem }()

			results[idx] = RunJob(cfg, e.job, e.job.ID)

			cost := int64(runResultTotalCost(results[idx]) * 10000)
			newTotal := runSpend.Add(cost)
			printRunSpend(e.job.ID, float64(newTotal)/10000)
		}(i, entry)
	}

	wg.Wait()

	// Summary
	fmt.Println()
	fmt.Println("GARY // YARD COMPLETE")
	fmt.Println()
	fmt.Printf("%-20s %-10s %-10s %s\n", "JOB", "STATUS", "ATTEMPTS", "BRANCH")
	fmt.Println(strings.Repeat("-", 60))

	done := 0
	failed := 0
	human := 0
	budgetStoppedCount := 0
	totalCost := 0.0
	quality := cost.ProviderReported
	for _, r := range results {
		status := r.FinalStatus
		branch := r.Branch
		if branch == "" {
			branch = "-"
		}
		attempts := fmt.Sprintf("%d", r.Attempts)
		fmt.Printf("%-20s %-10s %-10s %s\n", r.JobID, status, attempts, branch)

		totalCost += runResultTotalCost(r)
		if r.Cost == nil {
			quality = cost.LegacyEstimate
		} else if quality != cost.ProviderReported && quality != r.Cost.PricingStatus || quality == cost.ProviderReported && r.Cost.PricingStatus != cost.ProviderReported {
			quality = cost.Mixed
		}
		switch r.FinalStatus {
		case "DONE":
			done++
		case "HUMAN_REQUIRED":
			human++
		case "BUDGET_STOPPED":
			budgetStoppedCount++
		default:
			failed++
		}
	}

	fmt.Println()
	fmt.Printf("Done: %d\n", done)
	fmt.Printf("Failed: %d\n", failed)
	fmt.Printf("Human required: %d\n", human)
	if budgetStoppedCount > 0 {
		fmt.Printf("Budget stopped: %d\n", budgetStoppedCount)
	}
	fmt.Printf("Total cost: $%.4f USD\n", totalCost)
	fmt.Printf("Cost quality: %s\n", quality)
}

func runResultTotalCost(result jobs.RunResult) float64 {
	if result.Cost != nil {
		return result.Cost.TotalCostUSD
	}
	return result.TotalCostEstimated
}

func printRunSpend(jobID string, spend float64) {
	outputMu.Lock()
	defer outputMu.Unlock()
	fmt.Printf("[%s] Estimated run spend so far: $%.4f\n", jobID, spend)
}
