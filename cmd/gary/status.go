package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/config"
)

func cmdStatus() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config load failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("GARY // STATUS")
	fmt.Println()

	// Check for active Batch jobs
	activeJobs := checkActiveBatchJobs(cfg)
	if cfg.GCPProject != "" && cfg.GCPRegion != "" {
		if len(activeJobs) > 0 {
			fmt.Printf("Active jobs: %d\n", len(activeJobs))
			for _, j := range activeJobs {
				fmt.Printf("  %s\n", j)
			}
		} else {
			fmt.Println("Active jobs: 0")
		}
	} else {
		fmt.Println("Active jobs: unknown (no GCP config)")
	}

	// Cost summary from ledger
	costs := SummarizeCosts(cfg.ResultsDir, "")
	if costs.LastRunSet {
		fmt.Printf("Last run: %s (%s)\n", costs.LastRun.JobID, costs.LastRun.Status)
		fmt.Printf("Last result: %s\n", costs.LastRun.Status)
		fmt.Printf("Last run cost: $%.4f (%s)\n", costs.LastRun.TotalCost(), ledgerCostQuality(costs.LastRun))
	} else {
		fmt.Println("Last run: none")
	}
	fmt.Printf("Month-to-date cost: $%.4f\n", costs.MonthCost)
	fmt.Printf("Lifetime cost: $%.4f\n", costs.LifeCost)

	// Workers
	maxWorkers := 4
	if v := os.Getenv("GARY_MAX_WORKERS"); v != "" {
		if n, err := fmt.Sscanf(v, "%d", &maxWorkers); n == 1 && err == nil && maxWorkers >= 1 {
			// ok
		}
	}
	fmt.Printf("Workers: %d max\n", maxWorkers)

	// Budget
	if cfg.MaxCostUSD > 0 {
		fmt.Printf("Budget: $%.2f (pre-dispatch estimate)\n", cfg.MaxCostUSD)
	} else {
		fmt.Println("Budget: unlimited")
	}

	// Parked state
	if len(activeJobs) == 0 {
		fmt.Println("Cloud yard: IDLE")
	} else {
		fmt.Println("Cloud yard: ACTIVE")
	}
}

func ledgerCostQuality(entry LedgerEntry) string {
	if entry.Cost != nil {
		return entry.Cost.PricingStatus
	}
	return "LEGACY_ESTIMATE"
}

func checkActiveBatchJobs(cfg *config.Config) []string {
	if cfg.GCPProject == "" || cfg.GCPRegion == "" {
		return nil
	}

	args := []string{
		"batch", "jobs", "list",
		"--project", cfg.GCPProject,
		"--location", cfg.GCPRegion,
		"--format", "value(name,status.state)",
		"--filter", "status.state!=SUCCEEDED AND status.state!=FAILED AND status.state!=CANCELLED AND status.state!=DELETED",
	}

	out, err := exec.Command("gcloud", args...).CombinedOutput()
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var active []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			active = append(active, line)
		}
	}
	return active
}
