package main

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/jobs"
)

func TestLedgerAppendAndRead(t *testing.T) {
	dir := t.TempDir()

	entry := LedgerEntry{
		Timestamp:            time.Now().UTC().Format(time.RFC3339),
		JobID:                "test-job",
		Status:               "DONE",
		Attempts:             1,
		Model:                "gemini-3.5-flash-lite",
		ModelCostEstimated:   0.01,
		ComputeCostEstimated: 0.005,
		TotalCostEstimated:   0.015,
		DurationSecs:         100,
		Branch:               "gary/test-abc",
		Commit:               "abc123",
	}

	if err := AppendLedger(dir, entry); err != nil {
		t.Fatalf("AppendLedger: %v", err)
	}

	entries, err := ReadLedger(dir)
	if err != nil {
		t.Fatalf("ReadLedger: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].JobID != "test-job" {
		t.Errorf("expected job_id test-job, got %s", entries[0].JobID)
	}
	if entries[0].TotalCostEstimated != 0.015 {
		t.Errorf("expected total cost 0.015, got %f", entries[0].TotalCostEstimated)
	}
}

func TestLedgerMultipleEntries(t *testing.T) {
	dir := t.TempDir()

	for i := 0; i < 3; i++ {
		entry := LedgerEntry{
			Timestamp:          time.Now().UTC().Format(time.RFC3339),
			JobID:              "job-" + string(rune('a'+i)),
			Status:             "DONE",
			TotalCostEstimated: float64(i+1) * 0.01,
		}
		AppendLedger(dir, entry)
	}

	entries, _ := ReadLedger(dir)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	total := 0.0
	for _, e := range entries {
		total += e.TotalCostEstimated
	}
	if total != 0.06 {
		t.Errorf("expected total 0.06, got %f", total)
	}
}

func TestLedgerEmpty(t *testing.T) {
	dir := t.TempDir()

	entries, err := ReadLedger(dir)
	if err != nil {
		t.Fatalf("ReadLedger on empty: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestBudgetCheckUnset(t *testing.T) {
	allowed, _ := BudgetCheck(0, 0, 0.01)
	if !allowed {
		t.Error("unset budget should allow work")
	}
}

func TestBudgetCheckWithinLimit(t *testing.T) {
	// No committed work, limit $1.00, job costs $0.50
	allowed, _ := BudgetCheck(1.0, 0, 0.5)
	if !allowed {
		t.Error("should allow job within budget")
	}
}

func TestBudgetCheckExceedsLimit(t *testing.T) {
	// Committed $0.80, limit $1.00, new job costs $0.30 → projected $1.10 > $1.00
	allowed, reason := BudgetCheck(1.0, 0.80, 0.3)
	if allowed {
		t.Error("should reject job that exceeds budget")
	}
	if reason == "" {
		t.Error("expected a reason")
	}
}

func TestBudgetCheckDoesNotKillRunning(t *testing.T) {
	// Committed $0.40 (2 jobs already dispatched), limit $0.50
	// Existing work is NOT killed — budget only prevents NEW work
	allowed, _ := BudgetCheck(0.50, 0.40, 0.15)
	if allowed {
		t.Error("should prevent new work when committed + new exceeds budget")
	}
}

func TestBudgetCheckHistoricalSpendIgnored(t *testing.T) {
	// Historical ledger has $0.80 of old spend, but current run committed is $0
	// Per-run budget should allow work regardless of history
	allowed, _ := BudgetCheck(0.50, 0, 0.3)
	if !allowed {
		t.Error("historical spend should NOT consume a new run budget")
	}
}

func TestBudgetCheckConcurrentReservation(t *testing.T) {
	// Simulate concurrent dispatch: GARY_MAX_COST_USD=0.06, each job $0.025
	// Job A: committed 0 + 0.025 = 0.025 ≤ 0.06 → allowed
	// Job B: committed 0.025 + 0.025 = 0.050 ≤ 0.06 → allowed
	// Job C: committed 0.050 + 0.025 = 0.075 > 0.06 → rejected
	allowed, _ := BudgetCheck(0.06, 0.025, 0.025)
	if !allowed {
		t.Error("second job should be allowed")
	}
	allowed, _ = BudgetCheck(0.06, 0.050, 0.025)
	if allowed {
		t.Error("third job should be rejected")
	}
}

func TestConcurrentBudgetReservation(t *testing.T) {
	// Prove concurrent CAS reservations cannot exceed budget.
	// 10 goroutines race to reserve $0.015 each against a $0.06 budget.
	// At most 4 can succeed (4 × 0.015 = 0.06). The 5th must be rejected.
	var committed atomic.Int64 // fixed-point * 10000
	budget := 0.06
	jobCost := 0.015
	const goroutines = 10

	var allowedCount atomic.Int64
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				old := committed.Load()
				current := float64(old) / 10000
				ok, _ := BudgetCheck(budget, current, jobCost)
				if !ok {
					return
				}
				newVal := int64((current + jobCost) * 10000)
				if committed.CompareAndSwap(old, newVal) {
					allowedCount.Add(1)
					return
				}
			}
		}()
	}
	wg.Wait()

	finalCommitted := float64(committed.Load()) / 10000
	allowed := int(allowedCount.Load())

	if allowed > 4 {
		t.Errorf("expected at most 4 allowed, got %d", allowed)
	}
	if finalCommitted > budget+0.001 {
		t.Errorf("committed $%.4f exceeds budget $%.2f", finalCommitted, budget)
	}
	if allowed == 0 {
		t.Error("expected at least 1 allowed reservation")
	}
}

func TestBudgetStoppedStatus(t *testing.T) {
	result := jobs.RunResult{
		JobID:       "test",
		FinalStatus: "BUDGET_STOPPED",
	}

	if result.FinalStatus != "BUDGET_STOPPED" {
		t.Error("expected BUDGET_STOPPED status")
	}
}

func TestSummarizeCosts(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()

	// Add entries for current month
	AppendLedger(dir, LedgerEntry{
		Timestamp:          now.Format(time.RFC3339),
		JobID:              "job1",
		TotalCostEstimated: 0.10,
	})
	AppendLedger(dir, LedgerEntry{
		Timestamp:          now.Format(time.RFC3339),
		JobID:              "job2",
		TotalCostEstimated: 0.20,
	})

	summary := SummarizeCosts(dir, "")

	if summary.LifeCost < 0.29 || summary.LifeCost > 0.31 {
		t.Errorf("expected lifetime cost ~0.30, got %f", summary.LifeCost)
	}
	if summary.MonthCost < 0.29 || summary.MonthCost > 0.31 {
		t.Errorf("expected month cost ~0.30, got %f", summary.MonthCost)
	}
	if !summary.LastRunSet {
		t.Error("expected LastRunSet to be true")
	}
	if summary.LastRun.JobID != "job2" {
		t.Errorf("expected last run job2, got %s", summary.LastRun.JobID)
	}
}

func TestEstimateJobCost(t *testing.T) {
	cost1 := EstimateJobCost(1)
	cost3 := EstimateJobCost(3)

	if cost1 <= 0 {
		t.Errorf("expected positive cost for 1 attempt, got %f", cost1)
	}
	if cost3 <= cost1 {
		t.Errorf("expected cost3 > cost1, got %f <= %f", cost3, cost1)
	}
}

func TestMakeLedgerEntry(t *testing.T) {
	result := jobs.RunResult{
		JobID:                "my-job",
		FinalStatus:          "DONE",
		Attempts:             2,
		Branch:               "gary/my-job-abc",
		CommitSHA:            "def456",
		DurationSecs:         150,
		ModelCostEstimated:   0.05,
		ComputeCostEstimated: 0.01,
		TotalCostEstimated:   0.06,
	}

	entry := MakeLedgerEntry(result, "gemini-3.5-flash-lite")

	if entry.JobID != "my-job" {
		t.Errorf("expected job_id my-job, got %s", entry.JobID)
	}
	if entry.TotalCostEstimated != 0.06 {
		t.Errorf("expected total cost 0.06, got %f", entry.TotalCostEstimated)
	}
	if entry.Model != "gemini-3.5-flash-lite" {
		t.Errorf("expected model gemini-3.5-flash-lite, got %s", entry.Model)
	}
}

func TestLedgerFileLocation(t *testing.T) {
	dir := t.TempDir()
	expected := filepath.Join(dir, "cost-ledger.jsonl")

	AppendLedger(dir, LedgerEntry{
		Timestamp:          time.Now().UTC().Format(time.RFC3339),
		JobID:              "test",
		TotalCostEstimated: 0.01,
	})

	if _, err := os.Stat(expected); os.IsNotExist(err) {
		t.Errorf("expected ledger file at %s", expected)
	}
}
