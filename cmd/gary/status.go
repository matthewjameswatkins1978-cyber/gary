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
	fmt.Printf("Execution: single autonomous agent\n")
	fmt.Printf("Model:     %s\n", cfg.ImplementerModel)

	activeJobs := checkActiveBatchJobs(cfg)
	if cfg.GCPProject != "" && cfg.GCPRegion != "" {
		fmt.Printf("Active Batch jobs: %d\n", len(activeJobs))
		for _, j := range activeJobs {
			fmt.Printf("  %s\n", j)
		}
	} else {
		fmt.Println("Active Batch jobs: unknown (no GCP config)")
	}

	costs := SummarizeCosts(cfg.ResultsDir, "")
	if costs.LastRunSet {
		fmt.Printf("Last run:   %s (%s)\n", costs.LastRun.JobID, costs.LastRun.Status)
		fmt.Printf("Last cost:  $%.4f (%s)\n", costs.LastRun.TotalCost(), ledgerCostQuality(costs.LastRun))
	} else {
		fmt.Println("Last run:   none")
	}
	fmt.Printf("Month cost: $%.4f\n", costs.MonthCost)
	fmt.Printf("Life cost:  $%.4f\n", costs.LifeCost)

	if cfg.MaxCostUSD > 0 {
		fmt.Printf("Run budget: $%.2f pre-dispatch ceiling\n", cfg.MaxCostUSD)
	} else {
		fmt.Println("Run budget: not configured")
	}
	if len(activeJobs) == 0 {
		fmt.Println("Cloud:      IDLE")
	} else {
		fmt.Println("Cloud:      ACTIVE")
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
		if line = strings.TrimSpace(line); line != "" {
			active = append(active, line)
		}
	}
	return active
}
