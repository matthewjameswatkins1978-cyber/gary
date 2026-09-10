package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/appserver"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/config"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/yard"
)

type yardJobManifest struct {
	ID            string   `json:"id"`
	RepoPath      string   `json:"repo_path"`
	Repo          string   `json:"repo,omitempty"`
	BaseRef       string   `json:"base_ref"`
	WorkerProfile string   `json:"worker_profile,omitempty"`
	PacketPath    string   `json:"packet_path,omitempty"`
	Task          string   `json:"task,omitempty"`
	Acceptance    []string `json:"acceptance"`
	Preflight     string   `json:"preflight,omitempty"`
	StateDir      string   `json:"state_dir,omitempty"`
}

const yardmasterPrompt = `You are Luna, the persistent GARY Yardmaster brain.

Codex App Server owns your durable conversation, history, resume, approvals and input semantics. GARY tools are the only permitted implementation boundary. Read the repository WORKFLOW.md policy supplied below and obey it. Treat Git, the filesystem and deterministic acceptance as authoritative. End every response with exactly one line in this form:
GARY_STATUS: DONE|NEEDS_MATTHEW|FAILED

The current job packet follows.`

func cmdYardmaster() {
	fs := flag.NewFlagSet("work", flag.ContinueOnError)
	answer := fs.String("answer", "", "answer a persisted Matthew blocker and resume the same job")
	stateDirOverride := fs.String("state-dir", "", "local state directory (defaults to .gary-state beside the job manifest)")
	rawArgs := os.Args[2:]
	var flagArgs []string
	var positional []string
	for i := 0; i < len(rawArgs); i++ {
		arg := rawArgs[i]
		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
			if (arg == "--answer" || arg == "--state-dir") && i+1 < len(rawArgs) {
				i++
				flagArgs = append(flagArgs, rawArgs[i])
			}
			continue
		}
		positional = append(positional, arg)
	}
	if err := fs.Parse(append(flagArgs, positional...)); err != nil {
		os.Exit(2)
	}
	if len(positional) != 1 || fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "Usage: gary work <job.json> [--answer <text>] [--state-dir <dir>]")
		os.Exit(2)
	}
	manifestPath, _ := filepath.Abs(fs.Arg(0))
	manifest, err := loadYardJobManifest(manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "work: %v\n", err)
		os.Exit(1)
	}
	stateDir := *stateDirOverride
	if stateDir == "" {
		stateDir = manifest.StateDir
	}
	if stateDir == "" {
		stateDir = os.Getenv("GARY_STATE_DIR")
	}
	if stateDir == "" {
		stateDir = config.StateRoot()
	}
	manager, err := yard.NewManager(manifest.toSpec(), stateDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "work state: %v\n", err)
		os.Exit(1)
	}
	if err := manager.Reconcile(); err != nil {
		fmt.Fprintf(os.Stderr, "work reconcile: %v\n", err)
		os.Exit(1)
	}
	state := manager.State()
	if state.Status == "DONE" || state.Status == "FAILED" || state.Status == "CANCELLED" {
		fmt.Printf("GARY_STATUS: %s\n", state.Status)
		return
	}
	if state.Status == "NEEDS_MATTHEW" {
		if strings.TrimSpace(*answer) == "" {
			printMatthewBlocker(state)
			os.Exit(3)
		}
		if err := manager.ResolveBlocker(*answer); err != nil {
			fmt.Fprintf(os.Stderr, "work resume: %v\n", err)
			os.Exit(1)
		}
	}
	if err := manager.SetStatus("RUNNING"); err != nil {
		fmt.Fprintf(os.Stderr, "work state: %v\n", err)
		os.Exit(1)
	}

	logPath := filepath.Join(stateDir, "jobs", manifest.ID+".app-server.jsonl")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "work log: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client, err := appserver.Start(ctx, "codex", logFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "work app-server: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()
	if err := client.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "work initialize: %v\n", err)
		os.Exit(1)
	}
	model := os.Getenv("GARY_YARDMASTER_MODEL")
	if model == "" {
		model = "gpt-5.6-luna"
	}
	threadID, err := client.StartThread(manager.State().ThreadID, model, manifest.RepoPath, appserver.DynamicTools())
	if err != nil {
		fmt.Fprintf(os.Stderr, "work thread: %v\n", err)
		os.Exit(1)
	}
	if err := manager.SetThreadID(threadID); err != nil {
		fmt.Fprintf(os.Stderr, "work thread state: %v\n", err)
		os.Exit(1)
	}

	state = manager.State()
	input := buildYardmasterInput(manifest, state, *answer)
	for turn := 0; turn < 120; turn++ {
		if err := manager.IncrementYardmasterTurn(); err != nil {
			fmt.Fprintf(os.Stderr, "work state: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("\n[GARY yardmaster turn %d]\n", turn+1)
		var transcript bytes.Buffer
		progress := io.MultiWriter(os.Stdout, &transcript)
		turnStatus, err := client.Turn(ctx, threadID, input, yardToolHandler(manager), progress)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nwork turn: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("\n\nApp Server turn: %s\n", turnStatus)
		if transcript.Len() > 0 {
			fmt.Printf("\n\nLuna final status: %s\n", strings.TrimSpace(transcript.String()))
		}
		state = manager.State()
		if state.Status == "NEEDS_MATTHEW" {
			printMatthewBlocker(state)
			os.Exit(3)
		}
		if state.Worker != nil {
			view, statusErr := manager.WorkerStatus(state.Worker.ID)
			if statusErr == nil && view.ProcessAlive {
				event, waitErr := manager.WaitForWorkerEvent(ctx)
				if waitErr != nil {
					fmt.Fprintf(os.Stderr, "\nwork wait: %v\n", waitErr)
					os.Exit(1)
				}
				input = fmt.Sprintf("A mechanical GARY event has arrived for the same job; do not spawn a replacement. Reconcile reality and continue. EVENT: %s", eventSummary(event))
				continue
			}
			if statusErr == nil && !view.ProcessAlive && view.Record.Status == "RUNNING" {
				if err := manager.Reconcile(); err != nil {
					fmt.Fprintf(os.Stderr, "\nwork reconcile: %v\n", err)
					os.Exit(1)
				}
				event, waitErr := manager.WaitForWorkerEvent(ctx)
				if waitErr != nil {
					fmt.Fprintf(os.Stderr, "\nwork wait: %v\n", waitErr)
					os.Exit(1)
				}
				input = fmt.Sprintf("A mechanical GARY event has arrived for the same job; do not spawn a replacement. Reconcile reality and continue. EVENT: %s", eventSummary(event))
				continue
			}
			if statusErr == nil && view.Record.Status == "INTEGRATED" && strings.TrimSpace(view.DiffStat) != "" && !view.Dirty {
				_ = manager.SetStatus("DONE")
				printYardmasterDone(manager.State())
				return
			}
			if statusErr == nil && (view.Record.Status == "FAILED" || view.Record.Status == "ACCEPTANCE_FAILED") {
				_ = manager.SetStatus("FAILED")
				fmt.Printf("GARY_STATUS: FAILED\n")
				return
			}
		}
		// The model may have stopped after a useful observation. Give the same
		// durable thread one more turn; worker waiting itself is mechanical.
		input = "Continue the same job from the durable state. Reconcile the existing worker and finish with mechanically evidenced DONE, NEEDS_MATTHEW, or FAILED."
	}
	_ = manager.SetStatus("FAILED")
	fmt.Fprintln(os.Stderr, "work: Yardmaster turn limit reached without a terminal evidence state")
	os.Exit(2)
}

func loadYardJobManifest(path string) (*yardJobManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var m yardJobManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if m.RepoPath == "" {
		m.RepoPath = m.Repo
	}
	if !filepath.IsAbs(m.RepoPath) {
		m.RepoPath, _ = filepath.Abs(filepath.Join(filepath.Dir(path), m.RepoPath))
	}
	if m.PacketPath != "" {
		packetPath := m.PacketPath
		if !filepath.IsAbs(packetPath) {
			packetPath = filepath.Join(filepath.Dir(path), packetPath)
		}
		packet, err := os.ReadFile(packetPath)
		if err != nil {
			return nil, fmt.Errorf("read packet: %w", err)
		}
		m.Task = string(packet)
	}
	if strings.TrimSpace(m.ID) == "" || strings.TrimSpace(m.RepoPath) == "" || strings.TrimSpace(m.BaseRef) == "" || strings.TrimSpace(m.Task) == "" {
		return nil, errors.New("id, repo_path, base_ref and task are required")
	}
	if len(m.Acceptance) == 0 {
		return nil, errors.New("at least one acceptance command is required")
	}
	if _, err := os.Stat(m.RepoPath); err != nil {
		return nil, fmt.Errorf("repo_path: %w", err)
	}
	return &m, nil
}

func (m yardJobManifest) toSpec() yard.JobSpec {
	return yard.JobSpec{ID: m.ID, RepoPath: m.RepoPath, BaseRef: m.BaseRef, WorkerProfile: m.WorkerProfile, Task: m.Task, Acceptance: m.Acceptance, Preflight: m.Preflight}
}

func buildYardmasterInput(m *yardJobManifest, state yard.JobState, answer string) string {
	profile := m.WorkerProfile
	if profile == "" {
		profile = "not fixed; Luna may select the baseline profile"
	}
	return fmt.Sprintf("%s\n\nWORKFLOW POLICY:\n%s\n\nPREFLIGHT:\n%s\n\nJOB ID: %s\nTARGET REPOSITORY: %s\nBASE REF: %s\nREQUIRED WORKER PROFILE: %s\nACCEPTANCE:\n- %s\n\nWHOLE JOB PACKET:\n%s\n\nDURABLE GARY STATE:\n%s\n\n%s", yardmasterPrompt, loadWorkflow(m.RepoPath), m.Preflight, m.ID, m.RepoPath, m.BaseRef, profile, strings.Join(m.Acceptance, "\n- "), m.Task, stateSummary(state), answerSummary(answer))
}

func loadWorkflow(repo string) string {
	data, err := os.ReadFile(filepath.Join(repo, "WORKFLOW.md"))
	if err == nil && strings.TrimSpace(string(data)) != "" {
		return string(data)
	}
	return "Use the bounded GARY tools, preserve the packet, wait mechanically after dispatch, inspect reality, and never treat worker prose as acceptance."
}

func stateSummary(state yard.JobState) string {
	data, _ := json.MarshalIndent(state, "", "  ")
	return string(data)
}

func eventSummary(event yard.Event) string {
	data, _ := json.Marshal(event)
	return string(data)
}

func answerSummary(answer string) string {
	if strings.TrimSpace(answer) == "" {
		return "No new Matthew answer was supplied."
	}
	return "Matthew supplied this answer for the persisted boundary: " + answer
}

func yardToolHandler(manager *yard.Manager) appserver.ToolHandler {
	return func(name string, raw json.RawMessage) (string, error) {
		switch name {
		case "gary_spawn_worker":
			var in struct {
				Task        string `json:"task"`
				Profile     string `json:"profile"`
				Independent bool   `json:"independent"`
			}
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", err
			}
			view, err := manager.SpawnWorkerBounded(in.Task, in.Profile, in.Independent)
			return encodeToolResult(view, err)
		case "gary_worker_status":
			var in struct {
				WorkerID string `json:"worker_id"`
			}
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", err
			}
			view, err := manager.WorkerStatus(in.WorkerID)
			return encodeToolResult(view, err)
		case "gary_run_acceptance":
			var in struct {
				WorkerID string `json:"worker_id"`
			}
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", err
			}
			result, err := manager.RunAcceptance(in.WorkerID)
			return encodeToolResult(result, err)
		case "gary_inspect_diff":
			var in struct {
				WorkerID string `json:"worker_id"`
			}
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", err
			}
			diff, err := manager.InspectDiff(in.WorkerID)
			return diff, err
		case "gary_integrate_worker":
			var in struct {
				WorkerID string `json:"worker_id"`
			}
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", err
			}
			if err := manager.IntegrateWorker(in.WorkerID); err != nil {
				return "", err
			}
			return encodeToolResult(manager.State(), nil)
		case "gary_apply_small_fix":
			var in struct {
				WorkerID string `json:"worker_id"`
				Reason   string `json:"reason"`
				Patch    string `json:"patch"`
			}
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", err
			}
			if err := manager.ApplySmallFix(in.WorkerID, in.Reason, in.Patch); err != nil {
				return "", err
			}
			return encodeToolResult(manager.State(), nil)
		case "gary_request_matthew":
			var in struct {
				Summary       string `json:"summary"`
				Needed        string `json:"needed"`
				Evidence      string `json:"evidence"`
				UnblockAction string `json:"unblock_action"`
			}
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", err
			}
			blocker, err := manager.RequestMatthew(in.Summary, in.Needed, in.Evidence, in.UnblockAction)
			return encodeToolResult(blocker, err)
		default:
			return "", fmt.Errorf("unknown GARY tool %q", name)
		}
	}
}

func encodeToolResult(value any, err error) (string, error) {
	if err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	return string(data), err
}

func printMatthewBlocker(state yard.JobState) {
	if state.Blocker == nil {
		fmt.Println("GARY_STATUS: NEEDS_MATTHEW")
		return
	}
	fmt.Printf("\nNEEDS_MATTHEW\nAction: %s\nNeeded: %s\nEvidence: %s\nUnblock: %s\nGARY_STATUS: NEEDS_MATTHEW\n", state.Blocker.Summary, state.Blocker.Needed, state.Blocker.Evidence, state.Blocker.UnblockAction)
}

func printYardmasterDone(state yard.JobState) {
	if state.Worker == nil {
		fmt.Println("GARY_STATUS: DONE")
		return
	}
	view := state.Worker
	fmt.Printf("\nGARY_STATUS: DONE\nWorker: %s\nBranch: %s\nCommit: %s\nWorktree: %s\nAcceptance evidence: %s\n", view.ID, view.Branch, view.CommitSHA, view.Worktree, view.AcceptancePath)
}
