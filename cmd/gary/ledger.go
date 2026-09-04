package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/cost"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/jobs"
)

type LedgerEntry struct {
	Timestamp            string        `json:"timestamp"`
	JobID                string        `json:"job_id"`
	Status               string        `json:"status"`
	Attempts             int           `json:"attempts"`
	Model                string        `json:"model"`
	ModelCostEstimated   float64       `json:"model_cost_estimated"`
	ComputeCostEstimated float64       `json:"compute_cost_estimated"`
	TotalCostEstimated   float64       `json:"total_cost_estimated"`
	DurationSecs         float64       `json:"duration_seconds"`
	Branch               string        `json:"branch"`
	Commit               string        `json:"commit"`
	Cost                 *cost.Summary `json:"cost,omitempty"`
}

type CostSummary struct {
	RunCost    float64
	MonthCost  float64
	LifeCost   float64
	LastRun    LedgerEntry
	LastRunSet bool
}

func ledgerPath(resultsDir string) string { return filepath.Join(resultsDir, "cost-ledger.jsonl") }

func AppendLedger(resultsDir string, entry LedgerEntry) error {
	if err := os.MkdirAll(filepath.Dir(ledgerPath(resultsDir)), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(ledgerPath(resultsDir), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	_, err = f.Write(append(data, '\n'))
	return err
}

func ReadLedger(resultsDir string) ([]LedgerEntry, error) {
	f, err := os.Open(ledgerPath(resultsDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var entries []LedgerEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if scanner.Text() == "" {
			continue
		}
		var entry LedgerEntry
		if json.Unmarshal(scanner.Bytes(), &entry) == nil {
			entries = append(entries, entry)
		}
	}
	return entries, scanner.Err()
}

func SummarizeCosts(resultsDir, runID string) CostSummary {
	entries, _ := ReadLedger(resultsDir)
	monthPrefix := time.Now().Format("2006-01")
	var s CostSummary
	for _, e := range entries {
		total := e.TotalCost()
		s.LifeCost += total
		if len(e.Timestamp) >= 7 && e.Timestamp[:7] == monthPrefix {
			s.MonthCost += total
		}
		if runID != "" && len(e.Timestamp) >= len(runID) && e.Timestamp[:len(runID)] == runID {
			s.RunCost += total
		}
		s.LastRun, s.LastRunSet = e, true
	}
	return s
}

func (e LedgerEntry) TotalCost() float64 {
	if e.Cost != nil {
		return e.Cost.TotalCostUSD
	}
	return e.TotalCostEstimated
}

func BudgetCheck(maxCostUSD, currentRunCommitted, estimatedJobCost float64) (bool, string) {
	if maxCostUSD <= 0 {
		return true, ""
	}
	projected := currentRunCommitted + estimatedJobCost
	if projected > maxCostUSD {
		return false, fmt.Sprintf("ESTIMATED run committed $%.2f + job $%.2f = $%.2f exceeds limit $%.2f", currentRunCommitted, estimatedJobCost, projected, maxCostUSD)
	}
	return true, ""
}

func EstimateJobCost(attempts int) float64 {
	return EstimateJobCostForModel("gemini-3.8-flash", attempts)
}

// EstimateJobCostForModel is deliberately only a pre-dispatch guard. The
// persisted ledger uses provider usage where available and observed Batch time.
func EstimateJobCostForModel(model string, attempts int) float64 {
	if attempts < 1 {
		return 0
	}
	pt := cost.PriceForModel(model)
	modelCost := 0.0
	if pt.Known {
		const inputPerAttempt = 15000
		const cachedPerAttempt = 2000
		const outputPerAttempt = 7500
		modelCost = cost.ModelCost(pt,
			(inputPerAttempt-cachedPerAttempt)*attempts,
			cachedPerAttempt*attempts,
			outputPerAttempt*attempts,
		)
	}
	computeCost, _ := cost.ComputeCost("e2-small", "STANDARD", float64(attempts)*270.0)
	return modelCost + computeCost
}

func MakeLedgerEntry(result jobs.RunResult, model string) LedgerEntry {
	return LedgerEntry{
		Timestamp:            time.Now().UTC().Format(time.RFC3339),
		JobID:                result.JobID,
		Status:               result.FinalStatus,
		Attempts:             result.Attempts,
		Model:                model,
		ModelCostEstimated:   result.ModelCostEstimated,
		ComputeCostEstimated: result.ComputeCostEstimated,
		TotalCostEstimated:   result.TotalCostEstimated,
		DurationSecs:         result.DurationSecs,
		Branch:               result.Branch,
		Commit:               result.CommitSHA,
		Cost:                 result.Cost,
	}
}
