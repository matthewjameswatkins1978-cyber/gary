package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/config"
)

func cmdStatus() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config load failed: %v\n", err)
		os.Exit(1)
	}
	stateDir := os.Getenv("GARY_STATE_DIR")
	if stateDir == "" {
		stateDir = filepath.Join(".", ".gary-state")
	}
	stateDir, _ = filepath.Abs(stateDir)
	fmt.Println("GARY // LOCAL STATUS")
	fmt.Println()
	fmt.Printf("Runtime:   Luna + local Codex App Server\n")
	fmt.Printf("State:     %s\n", stateDir)
	files, _ := filepath.Glob(filepath.Join(stateDir, "jobs", "*.json"))
	if len(files) == 0 {
		fmt.Println("Jobs:      none")
	} else {
		fmt.Printf("Jobs:      %d\n", len(files))
		for _, path := range files {
			data, readErr := os.ReadFile(path)
			var state struct {
				Job struct {
					ID string `json:"id"`
				} `json:"job"`
				ThreadID string `json:"thread_id"`
				Status   string `json:"status"`
				Worker   *struct {
					ID     string `json:"id"`
					Status string `json:"status"`
				} `json:"worker"`
			}
			if readErr != nil || json.Unmarshal(data, &state) != nil {
				fmt.Printf("  %s: unreadable state\n", filepath.Base(path))
				continue
			}
			worker := "none"
			if state.Worker != nil {
				worker = state.Worker.ID + "/" + state.Worker.Status
			}
			fmt.Printf("  %s: %s thread=%s worker=%s\n", state.Job.ID, state.Status, shortThread(state.ThreadID), worker)
		}
	}
	costs := SummarizeCosts(cfg.ResultsDir, "")
	if costs.LastRunSet {
		fmt.Printf("Legacy cost ledger: $%.4f last=%s\n", costs.LastRun.TotalCost(), costs.LastRun.JobID)
	} else {
		fmt.Println("Legacy cost ledger: empty")
	}
	if os.Getenv("GARY_ENABLE_BATCH") == "1" {
		fmt.Println("Legacy Batch: enabled explicitly")
	} else {
		fmt.Println("Legacy Batch: disabled (set GARY_ENABLE_BATCH=1 only for old experiments)")
	}
}

func shortThread(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return "none"
	}
	if len(id) > 18 {
		return id[:18]
	}
	return id
}
