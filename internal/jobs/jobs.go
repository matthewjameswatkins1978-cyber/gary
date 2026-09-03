// Package jobs defines bounded worker-job and worker-result records.
package jobs

import "github.com/matthewjameswatkins1978-cyber/gary/internal/cost"

// Job is a small coding task dispatched to a worker.
type Job struct {
	ID                   string   `json:"id"`
	Repo                 string   `json:"repo"`
	BaseBranch           string   `json:"base_branch"`
	Task                 string   `json:"task"`
	Acceptance           []string `json:"acceptance"`
	EstimatedMinutes     int      `json:"estimated_minutes,omitempty"`
	ProjectID            string   `json:"project_id,omitempty"`
	MasterPacketDigest   string   `json:"master_packet_digest,omitempty"`
	ProjectManualVersion int      `json:"project_manual_version,omitempty"`
	ProjectContext       string   `json:"project_context,omitempty"`
}

// RunResult records the outcome of a single gary run invocation.
type RunResult struct {
	JobID            string  `json:"job_id"`
	BatchJobID       string  `json:"batch_job_id"`
	// BatchState is transport/execution-wrapper evidence, not product acceptance.
	BatchState       string  `json:"batch_state,omitempty"`
	// ArtifactPrefix preserves the durable worker evidence/checkpoint location even when no result is accepted.
	ArtifactPrefix   string  `json:"artifact_prefix,omitempty"`
	Branch           string  `json:"branch"`
	CommitSHA        string  `json:"commit_sha"`
	Accepted         bool    `json:"accepted"`
	// Verification records who actually awarded the pass. Accepted results are
	// integrable only when this is trusted-independent.
	Verification     string  `json:"verification,omitempty"`
	// QualityReview records the Yardmaster merge-worthiness verdict for child work.
	QualityReview    string  `json:"quality_review,omitempty"`
	DurationSecs     float64 `json:"duration_seconds"`
	EstimatedMinutes int     `json:"estimated_minutes,omitempty"`
	PatchBytes       int64   `json:"patch_bytes"`
	Attempts         int     `json:"attempts"`
	RepairCount      int     `json:"repair_count"`
	YardmasterRepaired bool  `json:"yardmaster_repaired,omitempty"`
	Escalated        bool    `json:"escalated"`
	CloserUsed       bool    `json:"closer_used"`
	FinalStatus      string  `json:"final_status"`
	// FailureReason is bounded mechanical evidence for the Yardmaster. It is
	// not an acceptance claim and must never contain worker prose alone.
	FailureReason        string  `json:"failure_reason,omitempty"`
	ProjectID            string  `json:"project_id,omitempty"`
	MasterPacketDigest   string  `json:"master_packet_digest,omitempty"`
	ProjectManualVersion int     `json:"project_manual_version,omitempty"`
	ModelCostEstimated   float64 `json:"model_cost_estimated"`
	ComputeCostEstimated float64 `json:"compute_cost_estimated"`
	TotalCostEstimated   float64 `json:"total_cost_estimated"`
	// Cost is the authoritative structured record for new runs. The legacy
	// *_estimated fields above remain so old consumers and status files decode.
	Cost *cost.Summary `json:"cost,omitempty"`
}
