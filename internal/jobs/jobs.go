// Package jobs defines the records shared by the one-agent work path.
package jobs

import "github.com/matthewjameswatkins1978-cyber/gary/internal/cost"

// Job is the complete coding task dispatched to one implementation agent.
type Job struct {
	ID                 string   `json:"id"`
	Repo               string   `json:"repo"`
	BaseBranch         string   `json:"base_branch"`
	Task               string   `json:"task"`
	Acceptance         []string `json:"acceptance"`
	EstimatedMinutes   int      `json:"estimated_minutes,omitempty"`
	MasterPacketDigest string   `json:"master_packet_digest,omitempty"`
}

// RunResult records the outcome of one gary work invocation.
type RunResult struct {
	JobID                string        `json:"job_id"`
	BatchJobID           string        `json:"batch_job_id"`
	BatchState           string        `json:"batch_state,omitempty"`
	ArtifactPrefix       string        `json:"artifact_prefix,omitempty"`
	Branch               string        `json:"branch"`
	CommitSHA            string        `json:"commit_sha"`
	Accepted             bool          `json:"accepted"`
	Verification         string        `json:"verification,omitempty"`
	DurationSecs         float64       `json:"duration_seconds"`
	PatchBytes           int64         `json:"patch_bytes"`
	Attempts             int           `json:"attempts"`
	FinalStatus          string        `json:"final_status"`
	FailureReason        string        `json:"failure_reason,omitempty"`
	MasterPacketDigest   string        `json:"master_packet_digest,omitempty"`
	ModelCostEstimated   float64       `json:"model_cost_estimated"`
	ComputeCostEstimated float64       `json:"compute_cost_estimated"`
	TotalCostEstimated   float64       `json:"total_cost_estimated"`
	Cost                 *cost.Summary `json:"cost,omitempty"`
}
