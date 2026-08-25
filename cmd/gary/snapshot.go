package main

import (
	"os"
	"path/filepath"
	"time"

	"github.com/matthewjameswatkins1978-cyber/gary/control"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/config"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/jobs"
)

// snapshotActiveJob records a job as active in the snapshot.
func snapshotActiveJob(jobID, batchJobID, repo, baseBranch, resultBranch string, state control.JobState, attempts int, startedAt string) {
	repoRoot := findRepoRoot()
	if repoRoot == "" {
		return
	}

	snap, _ := control.ReadSnapshot(repoRoot)

	// Remove from recent if present (moving to active)
	var recent []control.JobEntry
	for _, r := range snap.RecentJobs {
		if r.ID != jobID {
			recent = append(recent, r)
		}
	}

	// Add/update active
	found := false
	for i, a := range snap.ActiveJobs {
		if a.ID == jobID {
			snap.ActiveJobs[i] = control.JobEntry{
				ID:           jobID,
				BatchJobID:   batchJobID,
				State:        state,
				Repo:         repo,
				BaseBranch:   baseBranch,
				ResultBranch: resultBranch,
				Attempts:     attempts,
				StartedAt:    startedAt,
			}
			found = true
			break
		}
	}
	if !found {
		snap.ActiveJobs = append(snap.ActiveJobs, control.JobEntry{
			ID:           jobID,
			BatchJobID:   batchJobID,
			State:        state,
			Repo:         repo,
			BaseBranch:   baseBranch,
			ResultBranch: resultBranch,
			Attempts:     attempts,
			StartedAt:    startedAt,
		})
	}

	control.PublishSnapshot(repoRoot, snap.ActiveJobs, recent)
}

// snapshotJobComplete moves a job from active to recent in the snapshot.
func snapshotJobComplete(jobID string, result jobs.RunResult) {
	repoRoot := findRepoRoot()
	if repoRoot == "" {
		return
	}

	snap, _ := control.ReadSnapshot(repoRoot)

	// Remove from active
	var active []control.JobEntry
	for _, a := range snap.ActiveJobs {
		if a.ID != jobID {
			active = append(active, a)
		}
	}

	// Add to recent
	state := control.JobFailed
	if result.FinalStatus == "DONE" {
		state = control.JobAccepted
	} else if result.FinalStatus == "HUMAN_REQUIRED" {
		state = control.JobBlocked
	}

	summary := result.FinalStatus
	if result.CloserUsed {
		summary += " (closer)"
	}

	recent := append(snap.RecentJobs, control.JobEntry{
		ID:           jobID,
		BatchJobID:   result.BatchJobID,
		State:        state,
		ResultBranch: result.Branch,
		ResultCommit: result.CommitSHA,
		Attempts:     result.Attempts,
		EstCost:      result.TotalCostEstimated,
		Summary:      summary,
		FinishedAt:   time.Now().UTC().Format(time.RFC3339),
	})

	control.PublishSnapshot(repoRoot, active, recent)
}

// findRepoRoot walks up to find a .git directory.
func findRepoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func updateSnapshot(cfg *config.Config, activeJobs []control.JobEntry, recentJobs []control.JobEntry) {
	repoRoot := findRepoRoot()
	if repoRoot == "" {
		return
	}
	control.PublishSnapshot(repoRoot, activeJobs, recentJobs)
}
