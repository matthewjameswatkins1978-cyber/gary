// Package jobs defines bounded worker-job and worker-result records.
package jobs

import "github.com/matthewjameswatkins1978-cyber/gary/internal/cost"

// Job is a small coding task dispatched to a worker.
type Job struct {
	ID         string   `json:"id"`
	Repo       string   `json:"repo"`
	BaseBranch string   `json:"base_branch"`
	Task       string   `json:"task"`
	Acceptance []string `json:"acceptance"`
}

// RunResult records the outcome of a single gary run invocation.
type RunResult struct {
	JobID                string  `json:"job_id"`
	BatchJobID           string  `json:"batch_job_id"`
	Branch               string  `json:"branch"`
	CommitSHA            string  `json:"commit_sha"`
	Accepted             bool    `json:"accepted"`
	DurationSecs         float64 `json:"duration_seconds"`
	PatchBytes           int64   `json:"patch_bytes"`
	Attempts             int     `json:"attempts"`
	RepairCount          int     `json:"repair_count"`
	Escalated            bool    `json:"escalated"`
	CloserUsed           bool    `json:"closer_used"`
	FinalStatus          string  `json:"final_status"`
	ModelCostEstimated   float64 `json:"model_cost_estimated"`
	ComputeCostEstimated float64 `json:"compute_cost_estimated"`
	TotalCostEstimated   float64 `json:"total_cost_estimated"`
	// Cost is the authoritative structured record for new runs. The legacy
	// *_estimated fields above remain so old consumers and status files decode.
	Cost *cost.Summary `json:"cost,omitempty"`
}
