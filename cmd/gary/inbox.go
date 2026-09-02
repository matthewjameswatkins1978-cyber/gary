package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/config"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/githubapp"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/jobs"
)

const defaultInboxPath = "gary/inbox/pending"

var inboxID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,29}$`)
var inboxBranch = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,127}$`)
var inboxCommand = regexp.MustCompile(`^[A-Za-z0-9_./:=+@%,-\[\]]+( [A-Za-z0-9_./:=+@%,-\[\]]+)*$`)

func acceptanceCommandSegments(command string) []string {
	raw := strings.Split(command, "&&")
	parts := make([]string, 0, len(raw))
	for _, part := range raw {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func validateAcceptanceCommand(command string) bool {
	parts := acceptanceCommandSegments(command)
	if len(parts) == 0 || len(parts) > 12 {
		return false
	}
	for _, part := range parts {
		if !inboxCommand.MatchString(part) {
			return false
		}
	}
	return true
}

type inboxStatus struct {
	ID          string            `json:"id"`
	Manifest    string            `json:"manifest"`
	State       string            `json:"state"`
	UpdatedAt   string            `json:"updated_at"`
	BatchJobID  string            `json:"batch_job_id,omitempty"`
	Branch      string            `json:"branch,omitempty"`
	CommitSHA   string            `json:"commit_sha,omitempty"`
	Detail      string            `json:"detail,omitempty"`
	Transitions []inboxTransition `json:"transitions"`
}

type inboxTransition struct {
	State  string `json:"state"`
	At     string `json:"at"`
	Detail string `json:"detail,omitempty"`
}

type inboxEntry struct {
	job      *jobs.Job
	manifest string
	status   string
}

func cmdInbox() {
	if len(os.Args) < 3 || len(os.Args) > 4 {
		fmt.Fprintln(os.Stderr, "Usage: gary inbox <validate|dispatch> [pending-directory]")
		os.Exit(1)
	}

	mode := os.Args[2]
	pendingDir := defaultInboxPath
	if len(os.Args) == 4 {
		pendingDir = os.Args[3]
	}

	entries, err := readInbox(pendingDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "inbox: %v\n", err)
		os.Exit(1)
	}
	if mode == "validate" {
		fmt.Printf("GARY // INBOX VALID\nPending jobs: %d\n", len(entries))
		return
	}
	if mode != "dispatch" {
		fmt.Fprintln(os.Stderr, "Usage: gary inbox <validate|dispatch> [pending-directory]")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "inbox config: %v\n", err)
		os.Exit(1)
	}
	if cfg.GCPProject == "" {
		cfg.GCPProject = discoverProject()
	}
	if cfg.GCPProject == "" {
		fmt.Fprintln(os.Stderr, "inbox: no GCP project: set GARY_GCP_PROJECT")
		os.Exit(1)
	}
	if cfg.GCPRegion == "" {
		cfg.GCPRegion = "europe-west2"
	}

	failed := false
	for _, entry := range entries {
		if isFinalInboxState(entry.status) {
			fmt.Printf("%s: already %s\n", entry.job.ID, entry.status)
			continue
		}
		if entry.status == "running" {
			fmt.Printf("%s: blocked by existing running status\n", entry.job.ID)
			failed = true
			continue
		}

		if err := writeInboxStatus(pendingDir, entry, "queued", "accepted by inbox"); err != nil {
			fmt.Fprintf(os.Stderr, "%s: write queued status: %v\n", entry.job.ID, err)
			failed = true
			continue
		}
		if err := writeInboxStatus(pendingDir, entry, "running", "dispatching through Batch"); err != nil {
			fmt.Fprintf(os.Stderr, "%s: write running status: %v\n", entry.job.ID, err)
			failed = true
			continue
		}
		result := RunJob(cfg, entry.job, entry.job.ID)
		state, detail := inboxResultState(result.FinalStatus)
		if err := writeInboxResult(pendingDir, entry, state, detail, result); err != nil {
			fmt.Fprintf(os.Stderr, "%s: write status: %v\n", entry.job.ID, err)
			failed = true
			continue
		}
		if state != "accepted" {
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}

func readInbox(pendingDir string) ([]inboxEntry, error) {
	files, err := os.ReadDir(pendingDir)
	if err != nil {
		return nil, fmt.Errorf("read pending directory: %w", err)
	}

	var entries []inboxEntry
	seen := make(map[string]string)
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}
		manifest := filepath.Join(pendingDir, file.Name())
		job, err := loadInboxJob(manifest)
		if err != nil {
			return nil, err
		}
		if prior, ok := seen[job.ID]; ok {
			return nil, fmt.Errorf("duplicate job id %q in %s and %s", job.ID, prior, manifest)
		}
		seen[job.ID] = manifest
		status, err := readInboxStatus(pendingDir, job.ID)
		if err != nil {
			return nil, err
		}
		entries = append(entries, inboxEntry{job: job, manifest: filepath.ToSlash(manifest), status: status.State})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].manifest < entries[j].manifest })
	return entries, nil
}

func loadInboxJob(path string) (*jobs.Job, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var job jobs.Job
	if err := decoder.Decode(&job); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("parse %s: more than one JSON value", path)
	}
	if err := validateInboxJob(&job); err != nil {
		return nil, fmt.Errorf("validate %s: %w", path, err)
	}
	return &job, nil
}

func validateInboxJob(job *jobs.Job) error {
	if !inboxID.MatchString(job.ID) {
		return errors.New("id must use lowercase letters, digits, and hyphens (1-30 chars)")
	}
	if _, err := githubapp.ParseRepository(job.Repo); err != nil {
		return fmt.Errorf("invalid repo: %w", err)
	}
	if !inboxBranch.MatchString(job.BaseBranch) || strings.Contains(job.BaseBranch, "..") {
		return errors.New("base_branch contains unsupported characters")
	}
	if strings.TrimSpace(job.Task) == "" || len(job.Task) > 12000 {
		return errors.New("task must be non-empty and at most 12000 bytes")
	}
	if len(job.Acceptance) == 0 {
		return errors.New("acceptance must contain at least one direct command")
	}
	for _, command := range job.Acceptance {
		if !validateAcceptanceCommand(command) {
			return fmt.Errorf("acceptance command %q contains unsupported shell syntax", command)
		}
	}
	return nil
}

func inboxStatusPath(pendingDir, id string) string {
	return filepath.Join(filepath.Dir(pendingDir), "status", id+".json")
}

func readInboxStatus(pendingDir, id string) (*inboxStatus, error) {
	path := inboxStatusPath(pendingDir, id)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &inboxStatus{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read status %s: %w", path, err)
	}
	var status inboxStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("parse status %s: %w", path, err)
	}
	if status.ID != id {
		return nil, fmt.Errorf("status %s has mismatched id %q", path, status.ID)
	}
	return &status, nil
}

func writeInboxStatus(pendingDir string, entry inboxEntry, state, detail string) error {
	status, err := readInboxStatus(pendingDir, entry.job.ID)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	status.ID = entry.job.ID
	status.Manifest = entry.manifest
	status.State = state
	status.UpdatedAt = now
	status.Detail = detail
	status.Transitions = append(status.Transitions, inboxTransition{State: state, At: now, Detail: detail})
	return saveInboxStatus(pendingDir, status)
}

func saveInboxStatus(pendingDir string, status *inboxStatus) error {
	path := inboxStatusPath(pendingDir, status.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func writeInboxResult(pendingDir string, entry inboxEntry, state, detail string, result jobs.RunResult) error {
	status, err := readInboxStatus(pendingDir, entry.job.ID)
	if err != nil {
		return err
	}
	status.BatchJobID = result.BatchJobID
	status.Branch = result.Branch
	status.CommitSHA = result.CommitSHA
	now := time.Now().UTC().Format(time.RFC3339)
	status.State = state
	status.UpdatedAt = now
	status.Detail = detail
	status.Transitions = append(status.Transitions, inboxTransition{State: state, At: now, Detail: detail})
	return saveInboxStatus(pendingDir, status)
}

func isFinalInboxState(state string) bool {
	return state == "accepted" || state == "blocked" || state == "failed"
}

func inboxResultState(finalStatus string) (string, string) {
	if finalStatus == "DONE" {
		return "accepted", "local verification passed"
	}
	if finalStatus == "BUDGET_STOPPED" || finalStatus == "HUMAN_REQUIRED" || finalStatus == "BLOCKED" {
		return "blocked", "GARY stopped: " + finalStatus
	}
	return "failed", "GARY finished: " + finalStatus
}
