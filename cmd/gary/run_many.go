package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/config"
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

	// Load all jobs
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
	fmt.Println()

	// Run concurrently
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup
	results := make([]jobs.RunResult, len(entries))

	for i, entry := range entries {
		wg.Add(1)
		go func(idx int, e jobEntry) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			results[idx] = RunJob(cfg, e.job, e.job.ID)
		}(i, entry)
	}

	wg.Wait()

	// Print summary
	fmt.Println()
	fmt.Println("GARY // YARD COMPLETE")
	fmt.Println()
	fmt.Printf("%-20s %-10s %-10s %s\n", "JOB", "STATUS", "ATTEMPTS", "BRANCH")
	fmt.Println(strings.Repeat("-", 60))

	done := 0
	failed := 0
	human := 0
	for _, r := range results {
		status := r.FinalStatus
		branch := r.Branch
		if branch == "" {
			branch = "-"
		}
		attempts := fmt.Sprintf("%d", r.Attempts)
		fmt.Printf("%-20s %-10s %-10s %s\n", r.JobID, status, attempts, branch)

		switch r.FinalStatus {
		case "DONE":
			done++
		case "HUMAN_REQUIRED":
			human++
		default:
			failed++
		}
	}

	fmt.Println()
	fmt.Printf("Done: %d\n", done)
	fmt.Printf("Failed: %d\n", failed)
	fmt.Printf("Human required: %d\n", human)
}
