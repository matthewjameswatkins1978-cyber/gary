package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/benchmark"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/cost"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/jobs"
)

// LedgerEntry is one line in the JSONL cost ledger.
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

// CostSummary holds cumulative cost totals.
type CostSummary struct {
	RunCost    float64
	MonthCost  float64
	LifeCost   float64
	LastRun    LedgerEntry
	LastRunSet bool
}

func ledgerPath(resultsDir string) string {
	return filepath.Join(resultsDir, "cost-ledger.jsonl")
}

// AppendLedger writes one entry to the JSONL ledger.
func AppendLedger(resultsDir string, entry LedgerEntry) error {
	dir := filepath.Dir(ledgerPath(resultsDir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
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
	data = append(data, '\n')
	_, err = f.Write(data)
	return err
}

// ReadLedger reads all entries from the JSONL ledger.
func ReadLedger(resultsDir string) ([]LedgerEntry, error) {
	path := ledgerPath(resultsDir)
	f, err := os.Open(path)
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
		line := scanner.Text()
		if line == "" {
			continue
		}
		var entry LedgerEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, scanner.Err()
}

// SummarizeCosts computes run/month/lifetime cost totals from ledger entries.
// runID groups entries belonging to the current run (by prefix of timestamp).
func SummarizeCosts(resultsDir, runID string) CostSummary {
	entries, _ := ReadLedger(resultsDir)
	now := time.Now()
	monthPrefix := now.Format("2006-01")
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

		s.LastRun = e
		s.LastRunSet = true
	}

	return s
}

// TotalCost returns the structured total for new entries, or the legacy
// heuristic total for old entries. The latter remains an estimate by schema.
func (e LedgerEntry) TotalCost() float64 {
	if e.Cost != nil {
		return e.Cost.TotalCostUSD
	}
	return e.TotalCostEstimated
}

// BudgetCheck reports whether a new job can start within the per-run cost limit.
// currentRunCommitted is the estimated total cost of jobs already dispatched
// in this invocation. Month-to-date and lifetime spend are NOT considered.
func BudgetCheck(maxCostUSD, currentRunCommitted, estimatedJobCost float64) (bool, string) {
	if maxCostUSD <= 0 {
		return true, ""
	}

	projected := currentRunCommitted + estimatedJobCost
	if projected > maxCostUSD {
		return false, fmt.Sprintf("ESTIMATED run committed $%.2f + job $%.2f = $%.2f exceeds limit $%.2f",
			currentRunCommitted, estimatedJobCost, projected, maxCostUSD)
	}
	return true, ""
}

// EstimateJobCost produces a conservative ESTIMATED cost for one coding job attempt
// using the historical default worker model.
func EstimateJobCost(attempts int) float64 {
	return EstimateJobCostForModel("gemini-3.5-flash-lite", attempts)
}

// EstimateJobCostForModel estimates a job using the model actually requested.
func EstimateJobCostForModel(model string, attempts int) float64 {
	pt := benchmark.PriceTableForModel(model)

	// Conservative estimate per attempt:
	// ~15K input tokens, ~7.5K output tokens (incl. thinking), ~2K cached
	inputPerAttempt := 15000
	outputPerAttempt := 7500
	cachedPerAttempt := 2000

	modelCost := benchmark.EstimateCost(pt, inputPerAttempt*attempts, outputPerAttempt*attempts, 0, cachedPerAttempt*attempts)

	// Pre-dispatch compute estimate: typical attempt is ~270s. The rate itself
	// comes from the shared versioned compute table.
	computeCost, _ := cost.ComputeCost("e2-small", "STANDARD", float64(attempts)*270.0)

	return modelCost + computeCost
}

// MakeLedgerEntry creates a LedgerEntry from a RunResult.
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
