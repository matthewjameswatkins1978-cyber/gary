package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/config"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/cost"
)

// cmdCost reports persisted cost for one single-agent GARY work run.
func cmdCost() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "Usage: gary cost <job-id>")
		os.Exit(1)
	}
	id := os.Args[2]
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cost: config load failed: %v\n", err)
		os.Exit(1)
	}

	data, err := os.ReadFile(filepath.Join(cfg.ResultsDir, "runs", id+".json"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "cost: no persisted result for %s\n", id)
		os.Exit(1)
	}
	var result struct {
		FinalStatus        string        `json:"final_status"`
		TotalCostEstimated float64       `json:"total_cost_estimated"`
		Cost               *cost.Summary `json:"cost,omitempty"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		fmt.Fprintf(os.Stderr, "cost: invalid persisted result for %s: %v\n", id, err)
		os.Exit(1)
	}
	if result.Cost != nil {
		printCostLine(id, result.FinalStatus, result.Cost)
		return
	}
	fmt.Printf("%s\n%s\n$%.4f USD\nLEGACY_ESTIMATE\n", id, result.FinalStatus, result.TotalCostEstimated)
}

func printCostLine(id, state string, summary *cost.Summary) {
	if summary == nil {
		fmt.Printf("%s\n%s\n$0.0000 USD\nUNKNOWN\n", id, state)
		return
	}
	fmt.Printf("%s\n%s\n$%.4f USD\n%s\n", id, state, summary.TotalCostUSD, summary.PricingStatus)
}
