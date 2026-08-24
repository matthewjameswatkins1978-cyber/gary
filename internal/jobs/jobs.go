// Package jobs defines bounded worker-job and worker-result records.
package jobs

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
	JobID        string  `json:"job_id"`
	BatchJobID   string  `json:"batch_job_id"`
	Branch       string  `json:"branch"`
	CommitSHA    string  `json:"commit_sha"`
	Accepted     bool    `json:"accepted"`
	DurationSecs float64 `json:"duration_seconds"`
	PatchBytes   int64   `json:"patch_bytes"`
}
