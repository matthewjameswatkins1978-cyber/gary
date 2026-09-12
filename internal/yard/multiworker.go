package yard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// JobProgress is the mechanical whole-job view used by the Yardmaster loop.
// It deliberately summarizes worker state without making any planning decision.
type JobProgress struct {
	Total         int  `json:"total"`
	Running       int  `json:"running"`
	Integrated    int  `json:"integrated"`
	Failed        int  `json:"failed"`
	Pending       int  `json:"pending"`
	AllIntegrated bool `json:"all_integrated"`
}

// Progress reports the whole worker set. The legacy singular Worker pointer is
// never authoritative for whole-job completion once multiple workers exist.
func (m *Manager) Progress() JobProgress {
	m.mu.Lock()
	defer m.mu.Unlock()
	workers := m.workersLocked()
	progress := JobProgress{Total: len(workers)}
	for _, worker := range workers {
		switch worker.Status {
		case "RUNNING":
			progress.Running++
		case "INTEGRATED":
			progress.Integrated++
		case "FAILED", "ACCEPTANCE_FAILED", "CANCELLED":
			progress.Failed++
		default:
			progress.Pending++
		}
	}
	progress.AllIntegrated = progress.Total > 0 && progress.Integrated == progress.Total
	return progress
}

// SpawnWorkerForYardmaster is the strict multi-worker entry point. Repeating
// the same running task is idempotent, but a distinct second running task must
// be explicitly declared independent. SpawnWorker keeps its legacy single-
// worker compatibility semantics for older callers.
func (m *Manager) SpawnWorkerForYardmaster(task, profile string, independent bool) (WorkerView, error) {
	m.mu.Lock()
	active := 0
	for _, worker := range m.workersLocked() {
		if worker.Status != "RUNNING" {
			continue
		}
		active++
		if task == worker.Task {
			view := m.workerViewLocked(worker, true)
			m.mu.Unlock()
			return view, nil
		}
	}
	m.mu.Unlock()
	if active > 0 && !independent {
		return WorkerView{}, errors.New("a distinct second worker task must be declared independent")
	}
	return m.spawnWorker(task, profile, independent)
}

// WaitForAnyWorkerEvent blocks mechanically while any worker is healthy and
// RUNNING. It does not rely on the legacy singular Worker pointer, so one
// completed worker cannot cause model polling while another worker is active.
func (m *Manager) WaitForAnyWorkerEvent(ctx context.Context) (Event, error) {
	for {
		m.mu.Lock()
		workers := m.workersLocked()
		if len(workers) == 0 {
			m.mu.Unlock()
			return Event{}, errors.New("worker not found")
		}
		anyRunning := false
		deadRunning := false
		for _, worker := range workers {
			if worker.Status != "RUNNING" {
				continue
			}
			anyRunning = true
			if !processAlive(worker.PID) {
				deadRunning = true
			}
		}
		if !anyRunning {
			event := m.latestEventLocked()
			m.mu.Unlock()
			return event, nil
		}
		m.mu.Unlock()

		if deadRunning {
			if err := m.Reconcile(); err != nil {
				return Event{}, err
			}
		}

		timer := time.NewTimer(500 * time.Millisecond)
		select {
		case event := <-m.wake:
			timer.Stop()
			return event, nil
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return Event{}, ctx.Err()
		}
	}
}

type integrationCursor struct {
	Head       string
	WorkerID   string
	Integrated bool
}

// IntegrateWorkerSerial lands one independently accepted candidate onto the
// exact target head produced by the prior successful GARY integration. The
// durable integration cursor lives in the event journal, avoiding a second
// orchestration state system while still surviving restarts.
func (m *Manager) IntegrateWorkerSerial(workerID string) error {
	m.mu.Lock()
	worker := m.workerByIDLocked(workerID)
	if worker == nil {
		m.mu.Unlock()
		return errors.New("worker not found")
	}
	if worker.Integrated {
		m.mu.Unlock()
		return nil
	}
	if worker.Status != "ACCEPTED" {
		m.mu.Unlock()
		return fmt.Errorf("worker %s is not independently accepted", workerID)
	}
	job := m.state.Job
	workerCopy := *worker
	eventsPath := m.state.EventsPath
	priorIntegrated := 0
	for _, candidate := range m.workersLocked() {
		if candidate.ID != workerID && candidate.Integrated {
			priorIntegrated++
		}
	}
	m.mu.Unlock()

	if strings.TrimSpace(workerCopy.CommitSHA) == "" {
		return errors.New("accepted worker has no commit SHA")
	}
	if strings.TrimSpace(captureGit(job.RepoPath, "status", "--porcelain")) != "" {
		return errors.New("target repository is dirty; integration refused")
	}

	cursor := latestIntegrationCursor(eventsPath)
	currentHead := captureGit(job.RepoPath, "rev-parse", "HEAD")
	// Recover the narrow crash window where the durable integration event was
	// written but the matching state file save did not complete.
	if cursor.Integrated && cursor.WorkerID == workerID && cursor.Head != "" && currentHead == cursor.Head {
		m.mu.Lock()
		if current := m.workerByIDLocked(workerID); current != nil {
			current.Integrated = true
			current.Status = "INTEGRATED"
			m.state.Worker = current
			err := m.saveLocked()
			m.mu.Unlock()
			return err
		}
		m.mu.Unlock()
		return errors.New("worker disappeared from durable state during integration recovery")
	}

	expectedHead := workerCopy.BaseSHA
	if expectedHead == "" {
		expectedHead = captureGit(job.RepoPath, "rev-parse", job.BaseRef)
	}
	if cursor.Head != "" {
		expectedHead = cursor.Head
	} else if priorIntegrated > 0 {
		return errors.New("serial integration cursor is missing for an already-integrated job; refusing unsafe continuation")
	}
	if expectedHead != "" && currentHead != expectedHead {
		return fmt.Errorf("target HEAD %s no longer matches GARY integration head %s", currentHead, expectedHead)
	}

	commitRange := workerCopy.CommitSHA
	if workerCopy.BaseSHA != "" && workerCopy.BaseSHA != workerCopy.CommitSHA {
		commitRange = workerCopy.BaseSHA + ".." + workerCopy.CommitSHA
	}
	if err := runGit(job.RepoPath, "-c", "user.name=GARY Yardmaster", "-c", "user.email=gary@local", "cherry-pick", "--no-edit", commitRange); err != nil {
		_ = runGit(job.RepoPath, "cherry-pick", "--abort")
		if captureGit(job.RepoPath, "rev-parse", "HEAD") != expectedHead || strings.TrimSpace(captureGit(job.RepoPath, "status", "--porcelain")) != "" {
			_ = restoreIntegrationTarget(job.RepoPath, expectedHead)
		}
		return fmt.Errorf("integrate accepted worker: %w", err)
	}

	for _, command := range job.Acceptance {
		if code, output := runAcceptanceCommand(job.RepoPath, command); code != 0 {
			rollbackErr := restoreIntegrationTarget(job.RepoPath, expectedHead)
			if rollbackErr != nil {
				return fmt.Errorf("final acceptance failed after integration: %s: %s; rollback also failed: %v", command, output, rollbackErr)
			}
			return fmt.Errorf("final acceptance failed after integration: %s: %s; target restored to %s", command, output, expectedHead)
		}
	}
	integratedHead := captureGit(job.RepoPath, "rev-parse", "HEAD")
	if integratedHead == "" {
		_ = restoreIntegrationTarget(job.RepoPath, expectedHead)
		return errors.New("integration completed but target HEAD could not be read; target restored")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	current := m.workerByIDLocked(workerID)
	if current == nil {
		_ = restoreIntegrationTarget(job.RepoPath, expectedHead)
		return errors.New("worker disappeared from durable state during integration")
	}
	current.Integrated = true
	current.Status = "INTEGRATED"
	m.state.Worker = current
	event := Event{
		EventID:   "integration:" + workerID + ":" + integratedHead,
		JobID:     m.state.Job.ID,
		WorkerID:  workerID,
		Type:      "WORKER_INTEGRATED",
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Data: map[string]any{
			"candidate_commit_sha": workerCopy.CommitSHA,
			"prior_target_head":    expectedHead,
			"target_head":          integratedHead,
		},
	}
	if err := m.appendEventLocked(event); err != nil {
		current.Integrated = false
		current.Status = "ACCEPTED"
		_ = restoreIntegrationTarget(job.RepoPath, expectedHead)
		return err
	}
	if err := m.saveLocked(); err != nil {
		current.Integrated = false
		current.Status = "ACCEPTED"
		rollbackErr := restoreIntegrationTarget(job.RepoPath, expectedHead)
		rollbackEvent := Event{
			EventID:   "integration-rollback:" + workerID + ":" + integratedHead,
			JobID:     m.state.Job.ID,
			WorkerID:  workerID,
			Type:      "WORKER_INTEGRATION_ROLLED_BACK",
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			Data: map[string]any{
				"rolled_back_head": integratedHead,
				"target_head":      expectedHead,
			},
		}
		journalErr := m.appendEventLocked(rollbackEvent)
		if rollbackErr != nil || journalErr != nil {
			return fmt.Errorf("save integrated state: %v; rollback error: %v; rollback journal error: %v", err, rollbackErr, journalErr)
		}
		return err
	}
	return nil
}

func latestIntegrationCursor(path string) integrationCursor {
	data, err := os.ReadFile(path)
	if err != nil {
		return integrationCursor{}
	}
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	for i := len(lines) - 1; i >= 0; i-- {
		var event Event
		if json.Unmarshal(lines[i], &event) != nil {
			continue
		}
		if event.Type != "WORKER_INTEGRATED" && event.Type != "WORKER_INTEGRATION_ROLLED_BACK" {
			continue
		}
		head, ok := event.Data["target_head"].(string)
		if !ok || strings.TrimSpace(head) == "" {
			continue
		}
		return integrationCursor{
			Head:       strings.TrimSpace(head),
			WorkerID:   event.WorkerID,
			Integrated: event.Type == "WORKER_INTEGRATED",
		}
	}
	return integrationCursor{}
}

func restoreIntegrationTarget(repoPath, head string) error {
	if strings.TrimSpace(head) == "" {
		return errors.New("cannot restore integration target without a prior HEAD")
	}
	if err := runGit(repoPath, "reset", "--hard", head); err != nil {
		return err
	}
	// The target was proven clean before integration, so removing newly-created
	// untracked files is bounded restoration rather than deletion of user work.
	if err := runGit(repoPath, "clean", "-fd"); err != nil {
		return err
	}
	return nil
}
