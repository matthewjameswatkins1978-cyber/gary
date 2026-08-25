package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/benchmark"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/jobs"
)

// LedgerEntry is one line in the JSONL cost ledger.
type LedgerEntry struct {
	Timestamp            string  `json:"timestamp"`
	JobID                string  `json:"job_id"`
	Status               string  `json:"status"`
	Attempts             int     `json:"attempts"`
	Model                string  `json:"model"`
	ModelCostEstimated   float64 `json:"model_cost_estimated"`
	ComputeCostEstimated float64 `json:"compute_cost_estimated"`
	TotalCostEstimated   float64 `json:"total_cost_estimated"`
	DurationSecs         float64 `json:"duration_seconds"`
	Branch               string  `json:"branch"`
	Commit               string  `json:"commit"`
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
		s.LifeCost += e.TotalCostEstimated

		if len(e.Timestamp) >= 7 && e.Timestamp[:7] == monthPrefix {
			s.MonthCost += e.TotalCostEstimated
		}

		if runID != "" && len(e.Timestamp) >= len(runID) && e.Timestamp[:len(runID)] == runID {
			s.RunCost += e.TotalCostEstimated
		}

		s.LastRun = e
		s.LastRunSet = true
	}

	return s
}

// BudgetCheck reports whether a new job can start within the cost limit.
// Returns (allowed, reason).
func BudgetCheck(resultsDir string, maxCostUSD float64, estimatedJobCost float64) (bool, string) {
	if maxCostUSD <= 0 {
		return true, ""
	}

	entries, _ := ReadLedger(resultsDir)
	now := time.Now()
	monthPrefix := now.Format("2006-01")
	var monthCost float64

	for _, e := range entries {
		if len(e.Timestamp) >= 7 && e.Timestamp[:7] == monthPrefix {
			monthCost += e.TotalCostEstimated
		}
	}

	projected := monthCost + estimatedJobCost
	if projected > maxCostUSD {
		return false, fmt.Sprintf("ESTIMATED month spend $%.2f + job $%.2f = $%.2f exceeds limit $%.2f",
			monthCost, estimatedJobCost, projected, maxCostUSD)
	}
	return true, ""
}

// EstimateJobCost produces a conservative ESTIMATED cost for one coding job attempt.
// Uses benchmark price table and typical Gemini CLI token usage.
func EstimateJobCost(attempts int) float64 {
	pt := benchmark.DefaultPriceTable()

	// Conservative estimate per attempt:
	// ~15K input tokens, ~7.5K output tokens (incl. thinking), ~2K cached
	inputPerAttempt := 15000
	outputPerAttempt := 7500
	cachedPerAttempt := 2000

	modelCost := benchmark.EstimateCost(pt, inputPerAttempt*attempts, outputPerAttempt*attempts, 0, cachedPerAttempt*attempts)

	// Compute: e2-small ≈ $0.0207/hr ≈ $0.00000575/sec
	// Typical attempt: ~270s wall time
	computeCost := float64(attempts) * 270.0 * 0.00000575

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
	}
}
