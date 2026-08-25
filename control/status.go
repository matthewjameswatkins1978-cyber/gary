// Package control manages the GitHub-readable status snapshot.
// This file defines the snapshot types and read/write functions.
package control

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// YardState represents the overall yard status.
type YardState string

const (
	YardParked   YardState = "parked"
	YardRunning  YardState = "running"
	YardBlocked  YardState = "blocked"
)

// JobState represents an individual job's status.
type JobState string

const (
	JobQueued    JobState = "queued"
	JobRunning   JobState = "running"
	JobRetrying  JobState = "retrying"
	JobAccepted  JobState = "accepted"
	JobFailed    JobState = "failed"
	JobBlocked   JobState = "blocked"
	JobCancelled JobState = "cancelled"
)

// JobEntry is a concise snapshot of one job.
type JobEntry struct {
	ID            string   `json:"id"`
	BatchJobID    string   `json:"batch_job_id,omitempty"`
	State         JobState `json:"state"`
	Repo          string   `json:"repo,omitempty"`
	BaseBranch    string   `json:"base_branch,omitempty"`
	ResultBranch  string   `json:"result_branch,omitempty"`
	ResultCommit  string   `json:"result_commit,omitempty"`
	StartedAt     string   `json:"started_at,omitempty"`
	FinishedAt    string   `json:"finished_at,omitempty"`
	Attempts      int      `json:"attempts,omitempty"`
	EstCost       float64  `json:"est_cost,omitempty"`
	Summary       string   `json:"summary,omitempty"`
}

// Snapshot is the full status file written to gary/control/status.json.
type Snapshot struct {
	UpdatedAt  string     `json:"updated_at"`
	YardState  YardState  `json:"yard_state"`
	ActiveJobs []JobEntry `json:"active_jobs,omitempty"`
	RecentJobs []JobEntry `json:"recent_jobs,omitempty"`
}

const maxRecentJobs = 20

var (
	mu        sync.Mutex
	snapshot  Snapshot
)

// StatusPath returns the path to status.json relative to the repo root.
func StatusPath(repoRoot string) string {
	return filepath.Join(repoRoot, "gary", "control", "status.json")
}

// UpdateSnapshot writes a new snapshot to disk.
// activeJobs are currently running/queued jobs.
// recentJobs are completed jobs (bounded to maxRecentJobs).
func UpdateSnapshot(repoRoot string, activeJobs []JobEntry, recentJobs []JobEntry) error {
	mu.Lock()
	defer mu.Unlock()

	if len(recentJobs) > maxRecentJobs {
		recentJobs = recentJobs[len(recentJobs)-maxRecentJobs:]
	}

	snapshot = Snapshot{
		UpdatedAt:  time.Now().UTC().Format(time.RFC3339),
		YardState:  computeYardState(activeJobs),
		ActiveJobs: activeJobs,
		RecentJobs: recentJobs,
	}

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(StatusPath(repoRoot))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	return os.WriteFile(StatusPath(repoRoot), data, 0o644)
}

// ReadSnapshot reads the current snapshot from disk.
func ReadSnapshot(repoRoot string) (*Snapshot, error) {
	data, err := os.ReadFile(StatusPath(repoRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return &Snapshot{YardState: YardParked}, nil
		}
		return nil, err
	}

	var s Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}

	return &s, nil
}

func computeYardState(active []JobEntry) YardState {
	if len(active) == 0 {
		return YardParked
	}
	for _, j := range active {
		if j.State == JobBlocked {
			return YardBlocked
		}
	}
	return YardRunning
}

// ResetSnapshot clears the in-memory snapshot.
func ResetSnapshot() {
	mu.Lock()
	defer mu.Unlock()
	snapshot = Snapshot{}
}
