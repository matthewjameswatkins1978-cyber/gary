// Package yard is the local, inspectable tool layer behind the Luna
// Yardmaster. It owns worktrees and deterministic checks, not agent reasoning.
package yard

import (
	"bytes"
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

const StateVersion = 1

type JobSpec struct {
	ID            string   `json:"id"`
	RepoPath      string   `json:"repo_path"`
	BaseRef       string   `json:"base_ref"`
	WorkerProfile string   `json:"worker_profile,omitempty"`
	Task          string   `json:"task"`
	Acceptance    []string `json:"acceptance"`
}

type JobState struct {
	Version   int           `json:"version"`
	Job       JobSpec       `json:"job"`
	ThreadID  string        `json:"thread_id,omitempty"`
	Status    string        `json:"status"`
	Worker    *WorkerRecord `json:"worker,omitempty"`
	Blocker   *Blocker      `json:"blocker,omitempty"`
	CreatedAt string        `json:"created_at"`
	UpdatedAt string        `json:"updated_at"`
	StatePath string        `json:"state_path,omitempty"`
}

type WorkerRecord struct {
	ID                 string `json:"id"`
	JobID              string `json:"job_id"`
	Profile            string `json:"profile"`
	Provider           string `json:"provider,omitempty"`
	Harness            string `json:"harness,omitempty"`
	Protocol           string `json:"protocol,omitempty"`
	Endpoint           string `json:"endpoint,omitempty"`
	CredentialStore    string `json:"credential_store,omitempty"`
	BillingClass       string `json:"billing_class,omitempty"`
	Certification      string `json:"certification,omitempty"`
	IdentityConfidence string `json:"identity_confidence,omitempty"`
	Task               string `json:"task"`
	Worktree           string `json:"worktree"`
	Branch             string `json:"branch"`
	CommitSHA          string `json:"commit_sha,omitempty"`
	PID                int    `json:"pid"`
	LogPath            string `json:"log_path"`
	LastMessagePath    string `json:"last_message_path"`
	AcceptancePath     string `json:"acceptance_path"`
	Status             string `json:"status"`
	ExitCode           *int   `json:"exit_code,omitempty"`
	StartedAt          string `json:"started_at"`
	FinishedAt         string `json:"finished_at,omitempty"`
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
		return &Manager{state: state, stateDir: stateDir}, nil
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("open job state: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return &Manager{
		stateDir: stateDir,
		state:    JobState{Version: StateVersion, Job: JobSpec{ID: spec.ID, RepoPath: repo, BaseRef: spec.BaseRef, WorkerProfile: spec.WorkerProfile, Task: spec.Task, Acceptance: append([]string(nil), spec.Acceptance...)}, Status: "RUNNING", CreatedAt: now, UpdatedAt: now, StatePath: path},
	}, nil
}

func statePath(stateDir, id string) string {
	return filepath.Join(stateDir, "jobs", safeName(id, 100)+".json")
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

func (m *Manager) SetStatus(status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.Status = status
	return m.saveLocked()
}

func (m *Manager) ResolveBlocker(answer string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state.Blocker == nil {
		return errors.New("job has no pending Matthew blocker")
	}
	m.state.Blocker.ResolvedAt = time.Now().UTC().Format(time.RFC3339Nano)
	m.state.Status = "RUNNING"
	if strings.TrimSpace(answer) != "" {
		m.state.Job.Task += "\n\nMatthew supplied this unblock answer: " + answer
	}
	return m.saveLocked()
}

func (m *Manager) SpawnWorker(task, profile string) (WorkerView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state.Worker != nil {
		return m.workerViewLocked(m.state.Worker, true), nil
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
	m.state.Worker = &WorkerRecord{ID: workerID, JobID: m.state.Job.ID, Profile: profile, Provider: metadata.Provider, Harness: metadata.Harness, Protocol: metadata.Protocol, Endpoint: metadata.Endpoint, CredentialStore: metadata.CredentialStore, BillingClass: metadata.BillingClass, Certification: metadata.Certification, IdentityConfidence: metadata.IdentityConfidence, Task: task, Worktree: workerRoot, Branch: branch, PID: cmd.Process.Pid, LogPath: logPath, LastMessagePath: lastMessagePath, AcceptancePath: acceptancePath, Status: "RUNNING", StartedAt: now}
	m.state.Status = "RUNNING"
	if err := m.saveLocked(); err != nil {
		_ = cmd.Process.Kill()
		_ = logFile.Close()
		return WorkerView{}, err
	}
	go m.waitWorker(cmd, logFile, workerID)
	return m.workerViewLocked(m.state.Worker, true), nil
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
			Provider: "OpenAI", Harness: "Codex CLI", Protocol: "codex-native", Executable: "codex", LaunchMode: "codex", CredentialStore: "Codex local session", BillingClass: "included", Certification: "baseline",
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
	defer m.mu.Unlock()
	if m.state.Worker == nil || m.state.Worker.ID != workerID {
		return
	}
	finished := time.Now().UTC().Format(time.RFC3339Nano)
	m.state.Worker.FinishedAt = finished
	m.state.Worker.CommitSHA = captureGit(m.state.Worker.Worktree, "rev-parse", "HEAD")
	if cmd.ProcessState != nil {
		code := cmd.ProcessState.ExitCode()
		m.state.Worker.ExitCode = &code
		if err == nil {
			m.state.Worker.Status = "COMPLETED"
		} else {
			m.state.Worker.Status = "FAILED"
		}
	} else {
		m.state.Worker.Status = "FAILED"
	}
	_ = m.saveLocked()
}

func (m *Manager) WorkerStatus(workerID string) (WorkerView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state.Worker == nil || (workerID != "" && m.state.Worker.ID != workerID) {
		return WorkerView{}, errors.New("worker not found")
	}
	view := m.workerViewLocked(m.state.Worker, true)
	if !view.ProcessAlive && m.state.Worker.Status == "RUNNING" {
		m.state.Worker.Status = "INTERRUPTED"
		m.state.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		_ = m.saveLocked()
		view.Record.Status = "INTERRUPTED"
		view.Reconciled = true
	}
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
	if m.state.Worker == nil || m.state.Worker.ID != workerID {
		return "", errors.New("worker not found")
	}
	if !directoryExists(m.state.Worker.Worktree) {
		return "", errors.New("worker worktree is missing")
	}
	committed := captureGit(m.state.Worker.Worktree, "diff", m.state.Job.BaseRef+"...HEAD")
	working := captureGit(m.state.Worker.Worktree, "diff", "HEAD")
	return trimOutput("COMMITTED DIFF:\n"+committed+"\nUNCOMMITTED DIFF:\n"+working, 30000), nil
}

func (m *Manager) RunAcceptance(workerID string) (AcceptanceResult, error) {
	m.mu.Lock()
	if m.state.Worker == nil || m.state.Worker.ID != workerID {
		m.mu.Unlock()
		return AcceptanceResult{}, errors.New("worker not found")
	}
	worker := *m.state.Worker
	job := m.state.Job
	m.mu.Unlock()
	if processAlive(worker.PID) {
		return AcceptanceResult{}, errors.New("worker is still running; reconcile before acceptance")
	}
	result := AcceptanceResult{WorkerID: workerID, Passed: true, RecordedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	committedDiff := captureGit(worker.Worktree, "diff", job.BaseRef+"...HEAD")
	dirty := strings.TrimSpace(captureGit(worker.Worktree, "status", "--porcelain")) != ""
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
		if err := os.WriteFile(worker.AcceptancePath, data, 0o600); err != nil {
			return result, fmt.Errorf("write acceptance evidence: %w", err)
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.state.Worker != nil && m.state.Worker.ID == workerID {
			m.state.Worker.Status = "ACCEPTANCE_FAILED"
			_ = m.saveLocked()
		}
		return result, nil
	}
	for _, command := range job.Acceptance {
		code, output := runAcceptanceCommand(worker.Worktree, command)
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
	if err := os.WriteFile(worker.AcceptancePath, data, 0o600); err != nil {
		return result, fmt.Errorf("write acceptance evidence: %w", err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state.Worker != nil && m.state.Worker.ID == workerID {
		m.state.Worker.CommitSHA = captureGit(worker.Worktree, "rev-parse", "HEAD")
		if result.Passed {
			m.state.Worker.Status = "ACCEPTED"
		} else {
			m.state.Worker.Status = "ACCEPTANCE_FAILED"
		}
		_ = m.saveLocked()
	}
	return result, nil
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
	if err := m.saveLocked(); err != nil {
		return Blocker{}, err
	}
	return blocker, nil
}

func (m *Manager) saveLocked() error {
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
