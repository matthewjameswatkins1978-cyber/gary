// Package yard is the local, inspectable tool layer behind the Luna
// Yardmaster. It owns worktrees and deterministic checks, not agent reasoning.
package yard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const StateVersion = 2

type JobSpec struct {
	ID            string   `json:"id"`
	RepoPath      string   `json:"repo_path"`
	BaseRef       string   `json:"base_ref"`
	WorkerProfile string   `json:"worker_profile,omitempty"`
	Task          string   `json:"task"`
	Acceptance    []string `json:"acceptance"`
	Preflight     string   `json:"preflight,omitempty"`
}

type JobState struct {
	Version                  int             `json:"version"`
	Job                      JobSpec         `json:"job"`
	ThreadID                 string          `json:"thread_id,omitempty"`
	Status                   string          `json:"status"`
	Worker                   *WorkerRecord   `json:"worker,omitempty"`
	Workers                  []*WorkerRecord `json:"workers,omitempty"`
	Blocker                  *Blocker        `json:"blocker,omitempty"`
	CreatedAt                string          `json:"created_at"`
	UpdatedAt                string          `json:"updated_at"`
	StatePath                string          `json:"state_path,omitempty"`
	EventsPath               string          `json:"events_path,omitempty"`
	Amendments               []string        `json:"amendments,omitempty"`
	PreflightEstimatedWall   string          `json:"preflight_estimated_wall,omitempty"`
	EstimatedIncrementalCash string          `json:"estimated_incremental_cash,omitempty"`
	ActualElapsedSeconds     float64         `json:"actual_elapsed_seconds,omitempty"`
	ActualIncrementalCash    string          `json:"actual_incremental_cash,omitempty"`
	EconomicsSource          string          `json:"economics_source,omitempty"`
	YardmasterTurns          int             `json:"yardmaster_turns,omitempty"`
	MatthewInterventions     int             `json:"matthew_interventions,omitempty"`
}

type Event struct {
	EventID   string         `json:"event_id"`
	JobID     string         `json:"job_id"`
	Type      string         `json:"type"`
	WorkerID  string         `json:"worker_id,omitempty"`
	Timestamp string         `json:"timestamp"`
	Data      map[string]any `json:"data,omitempty"`
}

type WorkerRecord struct {
	ID                  string `json:"id"`
	JobID               string `json:"job_id"`
	Profile             string `json:"profile"`
	Provider            string `json:"provider,omitempty"`
	Harness             string `json:"harness,omitempty"`
	Protocol            string `json:"protocol,omitempty"`
	Endpoint            string `json:"endpoint,omitempty"`
	CredentialStore     string `json:"credential_store,omitempty"`
	BillingClass        string `json:"billing_class,omitempty"`
	Certification       string `json:"certification,omitempty"`
	IdentityConfidence  string `json:"identity_confidence,omitempty"`
	Task                string `json:"task"`
	Worktree            string `json:"worktree"`
	Branch              string `json:"branch"`
	BaseSHA             string `json:"base_sha,omitempty"`
	Integrated          bool   `json:"integrated,omitempty"`
	CommitSHA           string `json:"commit_sha,omitempty"`
	PID                 int    `json:"pid"`
	LogPath             string `json:"log_path"`
	LastMessagePath     string `json:"last_message_path"`
	AcceptancePath      string `json:"acceptance_path"`
	AcceptanceCommitSHA string `json:"acceptance_commit_sha,omitempty"`
	Status              string `json:"status"`
	ExitCode            *int   `json:"exit_code,omitempty"`
	StartedAt           string `json:"started_at"`
	FinishedAt          string `json:"finished_at,omitempty"`
}

type Blocker struct {
	Kind          string `json:"kind"`
	Summary       string `json:"summary"`
	Needed        string `json:"needed"`
	Evidence      string `json:"evidence"`
	UnblockAction string `json:"unblock_action"`
	ResolvedAt    string `json:"resolved_at,omitempty"`
}

type WorkerView struct {
	Record         WorkerRecord `json:"record"`
	ProcessAlive   bool         `json:"process_alive"`
	WorktreeExists bool         `json:"worktree_exists"`
	Head           string       `json:"head,omitempty"`
	Branch         string       `json:"branch,omitempty"`
	Dirty          bool         `json:"dirty"`
	DiffStat       string       `json:"diff_stat,omitempty"`
	LatestCommit   string       `json:"latest_commit,omitempty"`
	LogTail        string       `json:"log_tail,omitempty"`
	Reconciled     bool         `json:"reconciled"`
}

type AcceptanceResult struct {
	WorkerID         string          `json:"worker_id"`
	Passed           bool            `json:"passed"`
	CandidatePresent bool            `json:"candidate_present"`
	Commands         []CommandResult `json:"commands"`
	RecordedAt       string          `json:"recorded_at"`
}

type CommandResult struct {
	Command  string `json:"command"`
	ExitCode int    `json:"exit_code"`
	Output   string `json:"output"`
}

type Manager struct {
	mu       sync.Mutex
	state    JobState
	stateDir string
	wake     chan Event
}

func NewManager(spec JobSpec, stateDir string) (*Manager, error) {
	if strings.TrimSpace(spec.ID) == "" || strings.TrimSpace(spec.RepoPath) == "" || strings.TrimSpace(spec.BaseRef) == "" || strings.TrimSpace(spec.Task) == "" {
		return nil, errors.New("id, repo_path, base_ref and task are required")
	}
	if len(spec.Acceptance) == 0 {
		return nil, errors.New("at least one acceptance command is required")
	}
	repo, err := filepath.Abs(spec.RepoPath)
	if err != nil {
		return nil, fmt.Errorf("resolve repo_path: %w", err)
	}
	if _, err := os.Stat(repo); err != nil {
		return nil, fmt.Errorf("repo_path: %w", err)
	}
	stateDir, err = filepath.Abs(stateDir)
	if err != nil {
		return nil, fmt.Errorf("resolve state directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(stateDir, "jobs"), 0o755); err != nil {
		return nil, fmt.Errorf("create state directory: %w", err)
	}
	path := statePath(stateDir, spec.ID)
	if data, err := os.ReadFile(path); err == nil {
		var state JobState
		if err := json.Unmarshal(data, &state); err != nil {
			return nil, fmt.Errorf("read job state: %w", err)
		}
		if state.Job.ID != spec.ID || filepath.Clean(state.Job.RepoPath) != filepath.Clean(repo) || state.Job.BaseRef != spec.BaseRef || state.Job.WorkerProfile != spec.WorkerProfile {
			return nil, errors.New("existing job state conflicts with supplied job")
		}
		state.StatePath = path
		state.EventsPath = eventsPath(stateDir, spec.ID)
		if len(state.Workers) == 0 && state.Worker != nil {
			state.Workers = []*WorkerRecord{state.Worker}
		}
		if state.Worker == nil && len(state.Workers) > 0 {
			state.Worker = state.Workers[len(state.Workers)-1]
		}
		return &Manager{state: state, stateDir: stateDir, wake: make(chan Event, 4)}, nil
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("open job state: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	manager := &Manager{
		stateDir: stateDir,
		state:    JobState{Version: StateVersion, Job: JobSpec{ID: spec.ID, RepoPath: repo, BaseRef: spec.BaseRef, WorkerProfile: spec.WorkerProfile, Task: spec.Task, Acceptance: append([]string(nil), spec.Acceptance...), Preflight: spec.Preflight}, Status: "RUNNING", CreatedAt: now, UpdatedAt: now, StatePath: path, EventsPath: eventsPath(stateDir, spec.ID), PreflightEstimatedWall: "15-120 minutes (LOW confidence)", EstimatedIncrementalCash: "UNKNOWN", ActualIncrementalCash: "UNKNOWN", EconomicsSource: "UNKNOWN"},
		wake:     make(chan Event, 4),
	}
	manager.mu.Lock()
	if err := manager.appendEventLocked(Event{EventID: "job:" + spec.ID + ":created", JobID: spec.ID, Type: "JOB_CREATED", Timestamp: now}); err != nil {
		manager.mu.Unlock()
		return nil, err
	}
	if strings.TrimSpace(spec.Preflight) != "" {
		if err := manager.appendEventLocked(Event{EventID: "job:" + spec.ID + ":preflight", JobID: spec.ID, Type: "PREFLIGHT_COMPLETED", Timestamp: now, Data: map[string]any{"source": "front_door"}}); err != nil {
			manager.mu.Unlock()
			return nil, err
		}
	}
	if err := manager.saveLocked(); err != nil {
		manager.mu.Unlock()
		return nil, err
	}
	manager.mu.Unlock()
	return manager, nil
}

// LoadManager opens an existing job without requiring the caller to recreate
// the original packet. It is used by mechanical control commands.
func LoadManager(stateDir, jobID string) (*Manager, error) {
	stateDir, err := filepath.Abs(stateDir)
	if err != nil {
		return nil, err
	}
	path := statePath(stateDir, jobID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read job state: %w", err)
	}
	var state JobState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse job state: %w", err)
	}
	state.StatePath = path
	state.EventsPath = eventsPath(stateDir, state.Job.ID)
	if len(state.Workers) == 0 && state.Worker != nil {
		state.Workers = []*WorkerRecord{state.Worker}
	}
	if state.Worker == nil && len(state.Workers) > 0 {
		state.Worker = state.Workers[len(state.Workers)-1]
	}
	return &Manager{state: state, stateDir: stateDir, wake: make(chan Event, 4)}, nil
}

func statePath(stateDir, id string) string {
	return filepath.Join(stateDir, "jobs", safeName(id, 100)+".json")
}

func eventsPath(stateDir, id string) string {
	return filepath.Join(stateDir, "jobs", safeName(id, 100)+".events.jsonl")
}

func writerLockPath(stateDir, id string) string {
	return filepath.Join(stateDir, "jobs", safeName(id, 100)+".lock")
}

func acquireWriterLock(path string) (func(), error) {
	for attempt := 0; attempt < 2; attempt++ {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			_, _ = fmt.Fprintf(file, "%d\n", os.Getpid())
			_ = file.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
		data, readErr := os.ReadFile(path)
		pid, parseErr := strconv.Atoi(strings.TrimSpace(string(data)))
		if readErr == nil && parseErr == nil && processAlive(pid) {
			return nil, errors.New("job writer lock is held by another GARY process")
		}
		if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
			return nil, removeErr
		}
	}
	return nil, errors.New("could not acquire job writer lock")
}

func safeName(value string, max int) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-_")
	if out == "" {
		out = "job"
	}
	if len(out) > max {
		out = strings.TrimRight(out[:max], "-_")
	}
	return out
}

func (m *Manager) State() JobState {
	m.mu.Lock()
	defer m.mu.Unlock()
	return cloneState(m.state)
}

func (m *Manager) SetThreadID(threadID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.ThreadID = threadID
	return m.saveLocked()
}

func (m *Manager) IncrementYardmasterTurn() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.YardmasterTurns++
	return m.saveLocked()
}

func (m *Manager) SetStatus(status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.Status = status
	if status == "DONE" || status == "FAILED" || status == "CANCELLED" {
		if created, err := time.Parse(time.RFC3339Nano, m.state.CreatedAt); err == nil {
			m.state.ActualElapsedSeconds = time.Since(created).Seconds()
		}
		if m.state.ActualIncrementalCash == "" {
			m.state.ActualIncrementalCash = "UNKNOWN"
		}
		if m.state.EconomicsSource == "" {
			m.state.EconomicsSource = "UNKNOWN"
		}
	}
	if status == "DONE" || status == "FAILED" {
		typ := "JOB_DONE"
		if status == "FAILED" {
			typ = "JOB_FAILED"
		}
		if err := m.appendEventLocked(Event{EventID: "job:" + m.state.Job.ID + ":" + strings.ToLower(status), JobID: m.state.Job.ID, Type: typ, Timestamp: time.Now().UTC().Format(time.RFC3339Nano)}); err != nil {
			return err
		}
	}
	return m.saveLocked()
}

// Cancel is a mechanical control operation. It stops known local workers but
// deliberately preserves their worktrees, branches and evidence.
func (m *Manager) Cancel() error {
	m.mu.Lock()
	if m.state.Status == "DONE" || m.state.Status == "FAILED" || m.state.Status == "CANCELLED" {
		m.mu.Unlock()
		return nil
	}
	for _, worker := range m.workersLocked() {
		if worker.Status == "RUNNING" {
			if processAlive(worker.PID) {
				if runtime.GOOS == "windows" {
					_ = exec.Command("taskkill", "/PID", strconv.Itoa(worker.PID), "/T", "/F").Run()
				} else if process, findErr := os.FindProcess(worker.PID); findErr == nil {
					_ = process.Kill()
				}
			}
			worker.Status = "CANCELLED"
			worker.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
		}
	}
	m.state.Status = "CANCELLED"
	if err := m.appendEventLocked(Event{EventID: "job:" + m.state.Job.ID + ":cancelled", JobID: m.state.Job.ID, Type: "JOB_CANCELLED", Timestamp: time.Now().UTC().Format(time.RFC3339Nano)}); err != nil {
		m.mu.Unlock()
		return err
	}
	err := m.saveLocked()
	m.mu.Unlock()
	return err
}

func (m *Manager) ResolveBlocker(answer string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state.Blocker == nil {
		return errors.New("job has no pending Matthew blocker")
	}
	m.state.Blocker.ResolvedAt = time.Now().UTC().Format(time.RFC3339Nano)
	m.state.Status = "RUNNING"
	m.state.MatthewInterventions++
	if strings.TrimSpace(answer) != "" {
		m.state.Job.Task += "\n\nMatthew supplied this unblock answer: " + answer
		m.state.Amendments = append(m.state.Amendments, answer)
	}
	if err := m.appendEventLocked(Event{EventID: fmt.Sprintf("matthew:%s:resume:%d", m.state.Job.ID, len(m.state.Amendments)), JobID: m.state.Job.ID, Type: "MATTHEW_RESUMED", Timestamp: time.Now().UTC().Format(time.RFC3339Nano)}); err != nil {
		return err
	}
	return m.saveLocked()
}

func (m *Manager) SpawnWorker(task, profile string) (WorkerView, error) {
	return m.spawnWorker(task, profile, false)
}

// SpawnWorkerBounded permits a second worker only when Luna explicitly marks
// the task independent. The default compatibility method remains single
// worker, so old callers cannot accidentally create parallel work.
func (m *Manager) SpawnWorkerBounded(task, profile string, independent bool) (WorkerView, error) {
	return m.spawnWorker(task, profile, independent)
}

func (m *Manager) spawnWorker(task, profile string, independent bool) (WorkerView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	workers := m.workersLocked()
	active := 0
	for _, worker := range workers {
		if worker.Status == "RUNNING" {
			active++
			if task == worker.Task {
				return m.workerViewLocked(worker, true), nil
			}
		}
	}
	if active > 0 && !independent {
		for _, worker := range workers {
			if worker.Status == "RUNNING" {
				return m.workerViewLocked(worker, true), nil
			}
		}
	}
	if active >= 2 {
		return WorkerView{}, errors.New("maximum of two active workers is already running")
	}
	if active > 0 && !independent {
		return WorkerView{}, errors.New("a second worker must be declared independent")
	}
	if profile == "" {
		profile = m.state.Job.WorkerProfile
	}
	if profile == "" {
		profile = "gpt-5.6-luna"
	}
	if required := strings.TrimSpace(m.state.Job.WorkerProfile); required != "" && profile != required {
		return WorkerView{}, fmt.Errorf("job requires worker profile %q; refusing substitution with %q", required, profile)
	}
	args, metadata, err := workerCommand(profile)
	if err != nil {
		return WorkerView{}, err
	}
	if err := validateProfileSelection(m.state.Job, profile, metadata); err != nil {
		return WorkerView{}, err
	}
	workerID := "worker-" + time.Now().UTC().Format("20060102-150405.000000000")
	workerRoot := filepath.Join(m.stateDir, "worktrees", safeName(m.state.Job.ID, 80)+"-"+safeName(workerID, 40))
	if err := os.MkdirAll(filepath.Dir(workerRoot), 0o755); err != nil {
		return WorkerView{}, fmt.Errorf("create worktree parent: %w", err)
	}
	branch := "gary/" + safeName(m.state.Job.ID, 50) + "-" + safeName(workerID, 30)
	baseSHA := captureGit(m.state.Job.RepoPath, "rev-parse", m.state.Job.BaseRef)
	if err := runGit(m.state.Job.RepoPath, "worktree", "add", "-b", branch, workerRoot, m.state.Job.BaseRef); err != nil {
		return WorkerView{}, fmt.Errorf("create worker worktree: %w", err)
	}
	logPath := filepath.Join(m.stateDir, "workers", workerID+".jsonl")
	lastMessagePath := filepath.Join(m.stateDir, "workers", workerID+".last-message.txt")
	acceptancePath := filepath.Join(m.stateDir, "workers", workerID+".acceptance.json")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return WorkerView{}, fmt.Errorf("create worker evidence directory: %w", err)
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return WorkerView{}, fmt.Errorf("open worker log: %w", err)
	}
	workerTask := buildWorkerPrompt(task, m.state.Job)
	cmd := buildWorkerCommand(args, metadata, workerRoot, lastMessagePath, workerTask)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return WorkerView{}, fmt.Errorf("start Codex worker: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	worker := &WorkerRecord{ID: workerID, JobID: m.state.Job.ID, Profile: profile, Provider: metadata.Provider, Harness: metadata.Harness, Protocol: metadata.Protocol, Endpoint: metadata.Endpoint, CredentialStore: metadata.CredentialStore, BillingClass: metadata.BillingClass, Certification: metadata.Certification, IdentityConfidence: metadata.IdentityConfidence, Task: task, Worktree: workerRoot, Branch: branch, BaseSHA: baseSHA, PID: cmd.Process.Pid, LogPath: logPath, LastMessagePath: lastMessagePath, AcceptancePath: acceptancePath, Status: "RUNNING", StartedAt: now}
	m.state.Workers = append(workers, worker)
	m.state.Worker = worker
	m.state.Status = "RUNNING"
	if err := m.appendEventLocked(Event{EventID: "worker:" + workerID + ":started", JobID: m.state.Job.ID, WorkerID: workerID, Type: "WORKER_STARTED", Timestamp: now, Data: map[string]any{"profile": profile, "branch": branch, "base_ref": m.state.Job.BaseRef}}); err != nil {
		_ = cmd.Process.Kill()
		_ = logFile.Close()
		return WorkerView{}, err
	}
	if err := m.saveLocked(); err != nil {
		_ = cmd.Process.Kill()
		_ = logFile.Close()
		return WorkerView{}, err
	}
	go m.waitWorker(cmd, logFile, workerID)
	return m.workerViewLocked(worker, true), nil
}

func (m *Manager) workersLocked() []*WorkerRecord {
	if len(m.state.Workers) == 0 && m.state.Worker != nil {
		m.state.Workers = []*WorkerRecord{m.state.Worker}
	}
	return m.state.Workers
}

func (m *Manager) workerByIDLocked(workerID string) *WorkerRecord {
	workers := m.workersLocked()
	if workerID == "" && len(workers) > 0 {
		return workers[len(workers)-1]
	}
	for _, worker := range workers {
		if worker.ID == workerID {
			return worker
		}
	}
	return nil
}

type workerMetadata struct {
	Provider           string
	Harness            string
	Protocol           string
	Endpoint           string
	Executable         string
	LaunchMode         string
	CredentialStore    string
	BillingClass       string
	Certification      string
	IdentityConfidence string
	ExplicitOnly       bool
}

func validateProfileSelection(job JobSpec, profile string, metadata workerMetadata) error {
	if required := strings.TrimSpace(job.WorkerProfile); required != "" && profile != required {
		return fmt.Errorf("job requires worker profile %q; refusing substitution with %q", required, profile)
	}
	if metadata.ExplicitOnly && strings.TrimSpace(job.WorkerProfile) == "" {
		return fmt.Errorf("worker profile %q is candidate-only; set worker_profile explicitly", profile)
	}
	return nil
}

func workerCommand(profile string) ([]string, workerMetadata, error) {
	if profile == "" {
		profile = "gpt-5.6-luna"
	}
	switch profile {
	case "gpt-5.6-luna":
		return []string{"exec", "--json", "-m", profile}, workerMetadata{
			Provider: "OpenAI", Harness: "Codex CLI", Protocol: "codex-native", Executable: "codex", LaunchMode: "codex", CredentialStore: "Codex local session", BillingClass: "included", Certification: "CERTIFIED", IdentityConfidence: "HIGH",
		}, nil
	case "gpt-6-astra", "gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.5":
		return []string{"exec", "--json", "-m", profile}, workerMetadata{
			Provider: "OpenAI", Harness: "Codex CLI", Protocol: "codex-native", Executable: "codex", LaunchMode: "codex", CredentialStore: "Codex local session", BillingClass: "included", Certification: "candidate", IdentityConfidence: "UNVERIFIED", ExplicitOnly: true,
		}, nil
	case "opencode-go/muse-spark-1.3-contributor", "deepseek/deepseek-v4-flash", "cheaper-inference/deepseek-v4-flash":
		provider := map[string]string{
			"opencode-go/muse-spark-1.3-contributor": "OpenCode Go",
			"deepseek/deepseek-v4-flash":             "DeepSeek",
			"cheaper-inference/deepseek-v4-flash":    "cheaper-inference",
		}[profile]
		return []string{"run", "--format", "json", "--model", profile}, workerMetadata{
			Provider: provider, Harness: "OpenCode CLI", Protocol: "provider-managed", Executable: "opencode", LaunchMode: "opencode", CredentialStore: "OpenCode local auth", Certification: "candidate", IdentityConfidence: "UNVERIFIED", ExplicitOnly: true,
		}, nil
	case "antigravity/gemini-3.8-flash-high", "antigravity/gemini-3.8-flash-medium", "antigravity/gemini-3.8-flash-low":
		model := strings.TrimPrefix(profile, "antigravity/")
		return []string{"--output-format", "json", "--model", model}, workerMetadata{
			Provider: "Google Antigravity", Harness: "AGY CLI", Protocol: "local-cli", Executable: "agy", LaunchMode: "antigravity", CredentialStore: "Antigravity local session", BillingClass: "included quota", Certification: "candidate", IdentityConfidence: "LOCAL_SESSION", ExplicitOnly: true,
		}, nil
	case "mimo-v2.5":
		if strings.TrimSpace(os.Getenv("MIMO_API_KEY")) == "" {
			return nil, workerMetadata{}, errors.New("worker profile mimo-v2.5 unavailable: MIMO_API_KEY is not configured")
		}
		endpoint := strings.TrimSpace(os.Getenv("GARY_MIMO_BASE_URL"))
		if endpoint == "" {
			endpoint = "https://token-plan-ams.xiaomimimo.com/v1"
		}
		return []string{
				"exec", "--json", "-m", profile,
				"-c", "model_provider=\"mimo\"",
				"-c", "model_providers.mimo.name=\"mimo\"",
				"-c", "model_providers.mimo.base_url=\"" + endpoint + "\"",
				"-c", "model_providers.mimo.env_key=\"MIMO_API_KEY\"",
				"-c", "model_providers.mimo.wire_api=\"responses\"",
				"-c", "model_reasoning_effort=\"high\"",
				"-c", "model_supports_reasoning_summaries=true",
				"-c", "model_reasoning_summary=\"none\"",
				"-c", "model_context_window=1048576",
				"-c", "web_search=\"disabled\"",
			}, workerMetadata{
				Provider: "Xiaomi MiMo", Harness: "Codex CLI / Responses API", Protocol: "responses", Executable: "codex", LaunchMode: "codex", CredentialStore: "process env MIMO_API_KEY", Endpoint: endpoint,
				BillingClass: "Token Plan", Certification: "candidate", IdentityConfidence: "LOW", ExplicitOnly: true,
			}, nil
	default:
		return nil, workerMetadata{}, fmt.Errorf("unsupported worker profile %q; supported profiles are gpt-6-astra, gpt-5.6-sol, gpt-5.6-terra, gpt-5.6-luna, gpt-5.5, opencode-go/muse-spark-1.3-contributor, deepseek/deepseek-v4-flash, cheaper-inference/deepseek-v4-flash, antigravity/gemini-3.8-flash-high, antigravity/gemini-3.8-flash-medium, antigravity/gemini-3.8-flash-low and mimo-v2.5", profile)
	}
}

func buildWorkerCommand(args []string, metadata workerMetadata, workerRoot, lastMessagePath, task string) *exec.Cmd {
	if metadata.Executable == "" {
		metadata.Executable = "codex"
	}
	switch metadata.LaunchMode {
	case "opencode":
		args = append(args, "--dir", workerRoot, "--auto", task)
	case "antigravity":
		args = append(args, "--dangerously-skip-permissions", "--prompt", task)
	default:
		args = append(args, "-C", workerRoot, "-o", lastMessagePath, "--dangerously-bypass-approvals-and-sandbox", task)
	}
	cmd := exec.Command(metadata.Executable, args...)
	cmd.Dir = workerRoot
	return cmd
}

func buildWorkerPrompt(task string, job JobSpec) string {
	profileInstruction := "Use the worker profile selected by GARY."
	if job.WorkerProfile != "" {
		profileInstruction = fmt.Sprintf("GARY requires the exact worker profile %q. Do not substitute another model or provider.", job.WorkerProfile)
	}
	return fmt.Sprintf(`You are the only implementation worker for a GARY local prototype job.

Work only in the supplied current git worktree. Own this bounded task, inspect first, implement the smallest maintainable change, run the declared checks, and leave the verifiable changes in the worktree. Do not create another agent, supervisor, cloud job, or worktree. Do not push or modify credentials.

%s

TASK:
%s

ACCEPTANCE COMMANDS (GARY will rerun these independently):
%s

When complete, commit the intentional changes with a clear message and report what you actually verified. If an external human-only boundary is genuinely reached, stop and explain the exact action required.`, profileInstruction, task, strings.Join(job.Acceptance, "\n- "))
}

func (m *Manager) waitWorker(cmd *exec.Cmd, logFile *os.File, workerID string) {
	err := cmd.Wait()
	_ = logFile.Close()
	m.mu.Lock()
	worker := m.workerByIDLocked(workerID)
	if worker == nil {
		m.mu.Unlock()
		return
	}
	finished := time.Now().UTC().Format(time.RFC3339Nano)
	worker.FinishedAt = finished
	worker.CommitSHA = captureGit(worker.Worktree, "rev-parse", "HEAD")
	if cmd.ProcessState != nil {
		code := cmd.ProcessState.ExitCode()
		worker.ExitCode = &code
		if err == nil {
			worker.Status = "COMPLETED"
		} else {
			worker.Status = "FAILED"
		}
	} else {
		worker.Status = "FAILED"
	}
	m.state.Worker = worker
	exitData := map[string]any{"exit_code": -1, "commit_sha": worker.CommitSHA}
	if worker.ExitCode != nil {
		exitData["exit_code"] = *worker.ExitCode
	}
	_ = m.appendEventLocked(Event{EventID: "worker:" + workerID + ":exit", JobID: m.state.Job.ID, WorkerID: workerID, Type: "WORKER_EXITED", Timestamp: finished, Data: exitData})
	_ = m.saveLocked()
	m.mu.Unlock()
	_, _ = m.RunAcceptance(workerID)
	m.signalLatestEvent(workerID)
}

// Reconcile checks the saved process and workspace state before any new
// Yardmaster turn or worker spawn. It performs no model work.
func (m *Manager) Reconcile() error {
	m.mu.Lock()
	var lost []string
	for _, worker := range m.workersLocked() {
		if worker.Status == "RUNNING" && !processAlive(worker.PID) {
			finished := time.Now().UTC().Format(time.RFC3339Nano)
			worker.Status = "INTERRUPTED"
			worker.FinishedAt = finished
			worker.CommitSHA = captureGit(worker.Worktree, "rev-parse", "HEAD")
			_ = m.appendEventLocked(Event{EventID: "worker:" + worker.ID + ":exit", JobID: m.state.Job.ID, WorkerID: worker.ID, Type: "WORKER_EXITED", Timestamp: finished, Data: map[string]any{"recovered": true, "commit_sha": worker.CommitSHA}})
			lost = append(lost, worker.ID)
		}
	}
	if len(lost) == 0 {
		m.mu.Unlock()
		return nil
	}
	_ = m.saveLocked()
	m.mu.Unlock()
	for _, workerID := range lost {
		_, _ = m.RunAcceptance(workerID)
		m.signalLatestEvent(workerID)
	}
	return nil
}

// WaitForWorkerEvent blocks mechanically. A healthy worker causes no model
// turns; only a durable completion/reconciliation event releases the caller.
func (m *Manager) WaitForWorkerEvent(ctx context.Context) (Event, error) {
	for {
		m.mu.Lock()
		if m.state.Worker == nil {
			m.mu.Unlock()
			return Event{}, errors.New("worker not found")
		}
		if m.state.Worker.Status != "RUNNING" {
			event := m.latestEventLocked()
			m.mu.Unlock()
			return event, nil
		}
		alive := processAlive(m.state.Worker.PID)
		m.mu.Unlock()
		if !alive {
			_ = m.Reconcile()
			continue
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

func (m *Manager) signalLatestEvent(workerID string) {
	m.mu.Lock()
	event := m.latestEventLocked()
	if event.WorkerID == workerID {
		select {
		case m.wake <- event:
		default:
		}
	}
	m.mu.Unlock()
}

func (m *Manager) latestEventLocked() Event {
	data, err := os.ReadFile(m.state.EventsPath)
	if err != nil {
		return Event{}
	}
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	for i := len(lines) - 1; i >= 0; i-- {
		var event Event
		if json.Unmarshal(lines[i], &event) == nil {
			return event
		}
	}
	return Event{}
}

func (m *Manager) appendEventLocked(event Event) error {
	if event.EventID == "" {
		return errors.New("event_id is required")
	}
	event.JobID = m.state.Job.ID
	if event.Timestamp == "" {
		event.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	unlock, err := acquireWriterLock(writerLockPath(m.stateDir, m.state.Job.ID))
	if err != nil {
		return err
	}
	defer unlock()
	if _, err := os.Stat(m.state.EventsPath); err == nil {
		data, readErr := os.ReadFile(m.state.EventsPath)
		if readErr != nil {
			return readErr
		}
		for _, line := range bytes.Split(data, []byte("\n")) {
			var prior Event
			if json.Unmarshal(line, &prior) == nil && prior.EventID == event.EventID {
				return nil
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(m.state.EventsPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(append(data, '\n'))
	return err
}

func (m *Manager) WorkerStatus(workerID string) (WorkerView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	worker := m.workerByIDLocked(workerID)
	if worker == nil {
		return WorkerView{}, errors.New("worker not found")
	}
	view := m.workerViewLocked(worker, true)
	return view, nil
}

func (m *Manager) workerViewLocked(worker *WorkerRecord, reconciled bool) WorkerView {
	view := WorkerView{Record: *worker, ProcessAlive: processAlive(worker.PID), WorktreeExists: directoryExists(worker.Worktree), Reconciled: reconciled}
	if !view.WorktreeExists {
		return view
	}
	view.Branch = captureGit(worker.Worktree, "rev-parse", "--abbrev-ref", "HEAD")
	view.Head = captureGit(worker.Worktree, "rev-parse", "HEAD")
	view.Record.CommitSHA = view.Head
	view.Dirty = strings.TrimSpace(captureGit(worker.Worktree, "status", "--porcelain")) != ""
	view.DiffStat = captureGit(worker.Worktree, "diff", "--stat", m.state.Job.BaseRef+"...HEAD")
	view.LatestCommit = captureGit(worker.Worktree, "log", "-1", "--pretty=%s")
	view.LogTail = readTail(worker.LogPath, 6000)
	return view
}

func (m *Manager) InspectDiff(workerID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	worker := m.workerByIDLocked(workerID)
	if worker == nil {
		return "", errors.New("worker not found")
	}
	if !directoryExists(worker.Worktree) {
		return "", errors.New("worker worktree is missing")
	}
	committed := captureGit(worker.Worktree, "diff", m.state.Job.BaseRef+"...HEAD")
	working := captureGit(worker.Worktree, "diff", "HEAD")
	return trimOutput("COMMITTED DIFF:\n"+committed+"\nUNCOMMITTED DIFF:\n"+working, 30000), nil
}

func (m *Manager) RunAcceptance(workerID string) (AcceptanceResult, error) {
	m.mu.Lock()
	worker := m.workerByIDLocked(workerID)
	if worker == nil {
		m.mu.Unlock()
		return AcceptanceResult{}, errors.New("worker not found")
	}
	workerCopy := *worker
	job := m.state.Job
	m.mu.Unlock()
	if processAlive(workerCopy.PID) {
		return AcceptanceResult{}, errors.New("worker is still running; reconcile before acceptance")
	}
	head := captureGit(workerCopy.Worktree, "rev-parse", "HEAD")
	if workerCopy.AcceptanceCommitSHA != "" && workerCopy.AcceptanceCommitSHA == head {
		if data, err := os.ReadFile(workerCopy.AcceptancePath); err == nil {
			var cached AcceptanceResult
			if json.Unmarshal(data, &cached) == nil {
				return cached, nil
			}
		}
	}
	result := AcceptanceResult{WorkerID: workerID, Passed: true, RecordedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	committedDiff := captureGit(workerCopy.Worktree, "diff", job.BaseRef+"...HEAD")
	dirty := strings.TrimSpace(captureGit(workerCopy.Worktree, "status", "--porcelain")) != ""
	result.CandidatePresent = strings.TrimSpace(committedDiff) != "" && !dirty
	if !result.CandidatePresent {
		result.Passed = false
		result.Commands = append(result.Commands, CommandResult{
			Command:  "candidate diff and clean worktree",
			ExitCode: 1,
			Output:   "no committed candidate diff was found, or the worker worktree still has uncommitted changes",
		})
	}
	if !result.CandidatePresent {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return result, err
		}
		if err := os.WriteFile(workerCopy.AcceptancePath, data, 0o600); err != nil {
			return result, fmt.Errorf("write acceptance evidence: %w", err)
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		if current := m.workerByIDLocked(workerID); current != nil {
			current.Status = "ACCEPTANCE_FAILED"
			current.AcceptanceCommitSHA = head
			m.state.Worker = current
			_ = m.appendEventLocked(Event{EventID: "acceptance:" + workerID + ":none", JobID: m.state.Job.ID, WorkerID: workerID, Type: "ACCEPTANCE_COMPLETED", Timestamp: result.RecordedAt, Data: map[string]any{"passed": false, "candidate_present": false}})
			_ = m.saveLocked()
		}
		return result, nil
	}
	for _, command := range job.Acceptance {
		code, output := runAcceptanceCommand(workerCopy.Worktree, command)
		result.Commands = append(result.Commands, CommandResult{Command: command, ExitCode: code, Output: output})
		if code != 0 {
			result.Passed = false
			break
		}
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return result, err
	}
	if err := os.WriteFile(workerCopy.AcceptancePath, data, 0o600); err != nil {
		return result, fmt.Errorf("write acceptance evidence: %w", err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if current := m.workerByIDLocked(workerID); current != nil {
		current.CommitSHA = head
		current.AcceptanceCommitSHA = head
		if result.Passed {
			current.Status = "ACCEPTED"
		} else {
			current.Status = "ACCEPTANCE_FAILED"
		}
		m.state.Worker = current
		sha := current.CommitSHA
		_ = m.appendEventLocked(Event{EventID: "acceptance:" + workerID + ":" + sha, JobID: m.state.Job.ID, WorkerID: workerID, Type: "ACCEPTANCE_COMPLETED", Timestamp: result.RecordedAt, Data: map[string]any{"passed": result.Passed, "candidate_present": result.CandidatePresent}})
		_ = m.saveLocked()
	}
	return result, nil
}

// IntegrateWorker serially applies an independently accepted commit to the
// original checkout. It refuses a moving or dirty target so a worker can never
// silently land on an unexpected base.
func (m *Manager) IntegrateWorker(workerID string) error {
	m.mu.Lock()
	worker := m.workerByIDLocked(workerID)
	if worker == nil {
		m.mu.Unlock()
		return errors.New("worker not found")
	}
	if worker.Status != "ACCEPTED" {
		m.mu.Unlock()
		return fmt.Errorf("worker %s is not independently accepted", workerID)
	}
	if worker.Integrated {
		m.mu.Unlock()
		return nil
	}
	job := m.state.Job
	workerCopy := *worker
	m.mu.Unlock()
	if strings.TrimSpace(workerCopy.CommitSHA) == "" {
		return errors.New("accepted worker has no commit SHA")
	}
	if strings.TrimSpace(captureGit(job.RepoPath, "status", "--porcelain")) != "" {
		return errors.New("target repository is dirty; integration refused")
	}
	base := workerCopy.BaseSHA
	if base == "" {
		base = captureGit(job.RepoPath, "rev-parse", job.BaseRef)
	}
	if head := captureGit(job.RepoPath, "rev-parse", "HEAD"); base != "" && head != base {
		return fmt.Errorf("target HEAD %s no longer matches worker base %s", head, base)
	}
	commitRange := workerCopy.CommitSHA
	if base != "" && base != workerCopy.CommitSHA {
		commitRange = base + ".." + workerCopy.CommitSHA
	}
	if err := runGit(job.RepoPath, "cherry-pick", "--no-edit", commitRange); err != nil {
		return fmt.Errorf("integrate accepted worker: %w", err)
	}
	for _, command := range job.Acceptance {
		if code, output := runAcceptanceCommand(job.RepoPath, command); code != 0 {
			return fmt.Errorf("final acceptance failed after integration: %s: %s", command, output)
		}
	}
	m.mu.Lock()
	if current := m.workerByIDLocked(workerID); current != nil {
		current.Integrated = true
		current.Status = "INTEGRATED"
		m.state.Worker = current
		_ = m.appendEventLocked(Event{EventID: "integration:" + workerID + ":" + workerCopy.CommitSHA, JobID: m.state.Job.ID, WorkerID: workerID, Type: "INSPECTION_COMPLETED", Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Data: map[string]any{"integrated": true, "commit_sha": workerCopy.CommitSHA}})
		err := m.saveLocked()
		m.mu.Unlock()
		return err
	}
	m.mu.Unlock()
	return nil
}

// ApplySmallFix is the deliberately narrow Yardmaster repair boundary. It
// accepts only a small textual patch, proves it applies, reruns acceptance,
// and commits only after those checks pass.
func (m *Manager) ApplySmallFix(workerID, reason, patch string) error {
	if strings.TrimSpace(reason) == "" {
		return errors.New("small fix reason is required")
	}
	if strings.TrimSpace(patch) == "" {
		return errors.New("small fix patch is required")
	}
	if len(patch) > 8000 || strings.Contains(patch, "GIT binary patch") {
		return errors.New("REJECT SMALL_FIX: patch is too large or binary")
	}
	files, changed := patchStats(patch)
	if files == 0 || files > 2 || changed > 40 {
		return fmt.Errorf("REJECT SMALL_FIX: %d files and %d changed lines exceed the bounded repair limit", files, changed)
	}
	for _, file := range filesFromPatch(patch) {
		if filepath.IsAbs(file) || strings.Contains(file, "..") || strings.HasSuffix(file, ".lock") || strings.Contains(strings.ToLower(file), "vendor/") {
			return errors.New("REJECT SMALL_FIX: dependency or vendor file change")
		}
	}
	m.mu.Lock()
	worker := m.workerByIDLocked(workerID)
	if worker == nil {
		m.mu.Unlock()
		return errors.New("worker not found")
	}
	if worker.Status == "RUNNING" {
		m.mu.Unlock()
		return errors.New("worker is still running")
	}
	worktree := worker.Worktree
	job := m.state.Job
	m.mu.Unlock()
	check := exec.Command("git", "apply", "--check")
	check.Dir = worktree
	check.Stdin = strings.NewReader(patch)
	if output, err := check.CombinedOutput(); err != nil {
		return fmt.Errorf("REJECT SMALL_FIX: git apply --check: %s", strings.TrimSpace(string(output)))
	}
	apply := exec.Command("git", "apply", "--whitespace=error")
	apply.Dir = worktree
	apply.Stdin = strings.NewReader(patch)
	if output, err := apply.CombinedOutput(); err != nil {
		return fmt.Errorf("apply small fix: %s", strings.TrimSpace(string(output)))
	}
	for _, command := range job.Acceptance {
		if code, output := runAcceptanceCommand(worktree, command); code != 0 {
			return fmt.Errorf("small fix acceptance failed: %s", output)
		}
	}
	if err := runGit(worktree, "add", "-A"); err != nil {
		return err
	}
	if err := runGit(worktree, "-c", "user.name=GARY Yardmaster", "-c", "user.email=gary@local", "commit", "-m", "Apply bounded GARY small fix"); err != nil {
		return fmt.Errorf("commit small fix: %w", err)
	}
	if _, err := m.RunAcceptance(workerID); err != nil {
		return err
	}
	m.mu.Lock()
	if current := m.workerByIDLocked(workerID); current != nil {
		_ = m.appendEventLocked(Event{EventID: "small-fix:" + workerID + ":" + current.CommitSHA, JobID: m.state.Job.ID, WorkerID: workerID, Type: "SMALL_FIX_APPLIED", Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Data: map[string]any{"reason": reason, "files": files, "changed_lines": changed}})
		_ = m.saveLocked()
	}
	m.mu.Unlock()
	return nil
}

func filesFromPatch(patch string) []string {
	seen := map[string]bool{}
	var files []string
	for _, line := range strings.Split(patch, "\n") {
		if strings.HasPrefix(line, "+++ b/") {
			file := strings.TrimSpace(strings.TrimPrefix(line, "+++ b/"))
			if file != "/dev/null" && !seen[file] {
				seen[file] = true
				files = append(files, file)
			}
		}
	}
	return files
}

func patchStats(patch string) (int, int) {
	files := filesFromPatch(patch)
	changed := 0
	for _, line := range strings.Split(patch, "\n") {
		if len(line) > 0 && (line[0] == '+' || line[0] == '-') && !strings.HasPrefix(line, "+++") && !strings.HasPrefix(line, "---") {
			changed++
		}
	}
	return len(files), changed
}

func runAcceptanceCommand(dir, command string) (int, string) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		shell := "pwsh"
		if _, err := exec.LookPath(shell); err != nil {
			shell = "powershell"
		}
		cmd = exec.Command(shell, "-NoLogo", "-NoProfile", "-Command", command)
	} else {
		cmd = exec.Command("sh", "-lc", command)
	}
	cmd.Dir = dir
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	code := 0
	if err != nil {
		code = 1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		}
	}
	return code, trimOutput(output.String(), 12000)
}

func (m *Manager) RequestMatthew(summary, needed, evidence, unblockAction string) (Blocker, error) {
	if strings.TrimSpace(summary) == "" || strings.TrimSpace(needed) == "" || strings.TrimSpace(evidence) == "" || strings.TrimSpace(unblockAction) == "" {
		return Blocker{}, errors.New("summary, needed, evidence and unblock_action are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	blocker := Blocker{Kind: "human_boundary", Summary: summary, Needed: needed, Evidence: evidence, UnblockAction: unblockAction}
	m.state.Blocker = &blocker
	m.state.Status = "NEEDS_MATTHEW"
	data, err := json.MarshalIndent(blocker, "", "  ")
	if err != nil {
		return Blocker{}, err
	}
	if err := os.WriteFile(filepath.Join(m.stateDir, "jobs", safeName(m.state.Job.ID, 100)+".blocker.json"), data, 0o600); err != nil {
		return Blocker{}, fmt.Errorf("write blocker: %w", err)
	}
	if err := m.appendEventLocked(Event{EventID: "human:" + m.state.Job.ID + ":needed", JobID: m.state.Job.ID, Type: "NEEDS_MATTHEW", Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Data: map[string]any{"summary": summary, "needed": needed}}); err != nil {
		return Blocker{}, err
	}
	if err := m.saveLocked(); err != nil {
		return Blocker{}, err
	}
	return blocker, nil
}

func (m *Manager) saveLocked() error {
	unlock, err := acquireWriterLock(writerLockPath(m.stateDir, m.state.Job.ID))
	if err != nil {
		return err
	}
	defer unlock()
	m.state.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	data, err := json.MarshalIndent(m.state, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(m.state.StatePath), ".job-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, m.state.StatePath)
}

func cloneState(state JobState) JobState {
	data, _ := json.Marshal(state)
	var cloned JobState
	_ = json.Unmarshal(data, &cloned)
	return cloned
}

func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func captureGit(dir string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	if runtime.GOOS == "windows" {
		out, err := exec.Command("tasklist", "/FI", "PID eq "+strconv.Itoa(pid), "/FO", "CSV", "/NH").CombinedOutput()
		return err == nil && strings.Contains(string(out), strconv.Itoa(pid))
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}

func directoryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func readTail(path string, max int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return trimOutput(string(data), max)
}

func trimOutput(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) <= max {
		return value
	}
	return value[len(value)-max:]
}
