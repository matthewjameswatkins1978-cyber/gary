// Package control manages the GitHub-readable status snapshot.
package control

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/cost"
)

// YardState represents the overall yard status.
type YardState string

const (
	YardIdle YardState = "idle"
	// YardParked is retained as a source-compatible name for older callers.
	YardParked  YardState = YardIdle
	YardRunning YardState = "running"
	YardBlocked YardState = "blocked"
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
	ID                   string        `json:"id"`
	ProjectID            string        `json:"project_id,omitempty"`
	MasterPacketDigest   string        `json:"master_packet_digest,omitempty"`
	ProjectManualVersion int           `json:"project_manual_version,omitempty"`
	BatchJobID           string        `json:"batch_job_id,omitempty"`
	State                JobState      `json:"state"`
	Repo                 string        `json:"repo,omitempty"`
	BaseBranch           string        `json:"base_branch,omitempty"`
	ResultBranch         string        `json:"result_branch,omitempty"`
	ResultCommit         string        `json:"result_commit,omitempty"`
	StartedAt            string        `json:"started_at,omitempty"`
	FinishedAt           string        `json:"finished_at,omitempty"`
	Attempts             int           `json:"attempts,omitempty"`
	EstCost              float64       `json:"est_cost,omitempty"` // legacy compatibility
	Cost                 *cost.Summary `json:"cost,omitempty"`
	Summary              string        `json:"summary,omitempty"`
}

// Snapshot is the full status file written to control/status.json.
type Snapshot struct {
	UpdatedAt  string     `json:"updated_at"`
	YardState  YardState  `json:"yard_state"`
	ActiveJobs []JobEntry `json:"active_jobs,omitempty"`
	RecentJobs []JobEntry `json:"recent_jobs,omitempty"`
}

const maxRecentJobs = 20

var (
	mu       sync.Mutex
	snapshot Snapshot
)

// StatusPath returns the path to control/status.json relative to the repo root.
func StatusPath(repoRoot string) string {
	return filepath.Join(repoRoot, "control", "status.json")
}

// UpdateSnapshot writes a new snapshot to disk.
func UpdateSnapshot(repoRoot string, activeJobs []JobEntry, recentJobs []JobEntry) error {
	mu.Lock()
	defer mu.Unlock()

	// Callers may retry a terminal transition or race with a final worker
	// update. Never persist a terminal job in active_jobs.
	active := activeJobs[:0]
	for _, job := range activeJobs {
		if !isTerminalJobState(job.State) {
			active = append(active, job)
		}
	}

	if len(recentJobs) > maxRecentJobs {
		recentJobs = recentJobs[len(recentJobs)-maxRecentJobs:]
	}

	snapshot = Snapshot{
		UpdatedAt:  time.Now().UTC().Format(time.RFC3339),
		YardState:  computeYardState(active),
		ActiveJobs: active,
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

// PublishSnapshot writes the snapshot and commits it to the repository.
// Publishing failure must not fail the calling operation.
// Uses GARY_GITHUB_TOKEN env var for push auth when available.
func PublishSnapshot(repoRoot string, activeJobs []JobEntry, recentJobs []JobEntry) {
	if err := UpdateSnapshot(repoRoot, activeJobs, recentJobs); err != nil {
		return
	}

	token := os.Getenv("GARY_GITHUB_TOKEN")

	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoRoot
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=GARY",
			"GIT_AUTHOR_EMAIL=gary@agent-yard.local",
			"GIT_COMMITTER_NAME=GARY",
			"GIT_COMMITTER_EMAIL=gary@agent-yard.local",
		)
		_ = cmd.Run()
	}

	git("add", "control/status.json")
	// Bail silently if nothing to commit
	diff := exec.Command("git", "diff", "--cached", "--quiet", "control/status.json")
	diff.Dir = repoRoot
	if diff.Run() != nil {
		git("commit", "-m", "GARY: update status snapshot")
		// Push with token auth if available, otherwise try anonymous (local dev)
		if token != "" {
			pushURL := fmt.Sprintf("https://x-access-token:%s@github.com/%s.git", token, "matthewjameswatkins1978-cyber/gary")
			git("push", pushURL, "HEAD")
		} else {
			git("push", "origin", "HEAD")
		}
	}
}

// ReadSnapshot reads the current snapshot from disk.
func ReadSnapshot(repoRoot string) (*Snapshot, error) {
	data, err := os.ReadFile(StatusPath(repoRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return &Snapshot{YardState: YardIdle}, nil
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
		return YardIdle
	}
	for _, j := range active {
		if j.State == JobBlocked {
			return YardBlocked
		}
	}
	return YardRunning
}

func isTerminalJobState(state JobState) bool {
	switch state {
	case JobAccepted, JobFailed, JobBlocked, JobCancelled:
		return true
	case JobState("done"), JobState("budget_stopped"):
		return true
	default:
		return false
	}
}

// ResetSnapshot clears the in-memory snapshot.
func ResetSnapshot() {
	mu.Lock()
	defer mu.Unlock()
	snapshot = Snapshot{}
}
