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
	"time"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/appserver"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/yard"
)

type yardJobManifest struct {
	ID         string   `json:"id"`
	RepoPath   string   `json:"repo_path"`
	Repo       string   `json:"repo,omitempty"`
	BaseRef    string   `json:"base_ref"`
	PacketPath string   `json:"packet_path,omitempty"`
	Task       string   `json:"task,omitempty"`
	Acceptance []string `json:"acceptance"`
	StateDir   string   `json:"state_dir,omitempty"`
}

const yardmasterPrompt = `You are Luna, the persistent GARY Yardmaster brain.

GARY is deliberately a thin local yard layer. Codex App Server owns your durable conversation, history, resume, approvals and input semantics. The yard tools own only local git worktrees, one worker process, deterministic acceptance, evidence and a human-boundary record. Do not recreate a planner, swarm, wave scheduler, heartbeat protocol, telemetry reader, project board, retry engine or provider matrix.

Hold the whole supplied job together in this one durable thread. Use the yard tools when implementation work is needed. You may inspect the target through the yard tools, but do not implement the target yourself with built-in shell, file edits or apply-patch. Start at most one worker, and never spawn a replacement worker after an interruption: reconcile the existing process and worktree first.

A worker saying “done” is not acceptance. After the worker stops, inspect the actual worktree and run the declared acceptance tool. Treat the filesystem, git state and acceptance output as authoritative over worker prose. If acceptance fails, report FAILED with the evidence; do not invent success or silently retry. If a genuine human-only boundary is reached, call gary_request_matthew with the exact action, then stop and return NEEDS_MATTHEW. Do not ask Matthew for ordinary debugging, package installation or a first failed test.

Do not return while a worker is still running if another status check can make progress. When the job is mechanically accepted, return DONE with the worker id, branch, commit and acceptance evidence. End every response with exactly one line in this form:
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
		stateDir = filepath.Join(filepath.Dir(manifestPath), ".gary-state")
	}
	manager, err := yard.NewManager(manifest.toSpec(), stateDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "work state: %v\n", err)
		os.Exit(1)
	}
	state := manager.State()
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
				input = "The existing worker is still running. Stay on this same durable job, wait briefly, then call gary_worker_status again. Do not spawn another worker."
				time.Sleep(2 * time.Second)
				continue
			}
			if statusErr == nil && view.Record.Status == "ACCEPTED" && strings.TrimSpace(view.DiffStat) != "" && !view.Dirty {
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
		// durable thread one more turn rather than creating a controller loop.
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
	return yard.JobSpec{ID: m.ID, RepoPath: m.RepoPath, BaseRef: m.BaseRef, Task: m.Task, Acceptance: m.Acceptance}
}

func buildYardmasterInput(m *yardJobManifest, state yard.JobState, answer string) string {
	return fmt.Sprintf("%s\n\nJOB ID: %s\nTARGET REPOSITORY: %s\nBASE REF: %s\nACCEPTANCE:\n- %s\n\nWHOLE JOB PACKET:\n%s\n\nDURABLE GARY STATE:\n%s\n\n%s", yardmasterPrompt, m.ID, m.RepoPath, m.BaseRef, strings.Join(m.Acceptance, "\n- "), m.Task, stateSummary(state), answerSummary(answer))
}

func stateSummary(state yard.JobState) string {
	data, _ := json.MarshalIndent(state, "", "  ")
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
				Task    string `json:"task"`
				Profile string `json:"profile"`
			}
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", err
			}
			view, err := manager.SpawnWorker(in.Task, in.Profile)
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
