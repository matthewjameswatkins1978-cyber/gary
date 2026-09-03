package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/project"
)

func cmdProjectTick() {
	fs := flag.NewFlagSet("project tick", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	manifest := fs.String("manifest", "", "optional master manifest path")
	jsonOutput := fs.Bool("json", false, "machine-readable output")
	args, err := normalizeReviewArgs(os.Args[3:], map[string]bool{"--manifest": true})
	if err != nil || fs.Parse(args) != nil || fs.NArg() != 1 {
		printProjectUsage()
		os.Exit(1)
	}
	store := project.DefaultStore()
	now := time.Now().UTC()
	decision, err := store.ClaimAutoWake(fs.Arg(0), now, project.DefaultWatchdogPolicy())
	if err != nil {
		fmt.Fprintf(os.Stderr, "project tick: %v\n", err)
		os.Exit(1)
	}
	if decision.Action != project.WakeRequestedAction {
		printWakeDecision(decision, *jsonOutput)
		return
	}
	ctx, err := store.LoadContext(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "project tick: load claimed project: %v\n", err)
		os.Exit(1)
	}
	before := ctx.ProgressFingerprint()
	if err := store.StartAutoWake(fs.Arg(0), decision.WakeID, time.Now().UTC()); err != nil {
		fmt.Fprintf(os.Stderr, "project tick: start wake: %v\n", err)
		os.Exit(1)
	}
	manifestPath, err := findProjectMasterManifest(fs.Arg(0), *manifest)
	if err != nil {
		_ = store.RecordAutoWakeOutcome(fs.Arg(0), decision.WakeID, time.Now().UTC(), project.WakeOutcome{Failed: true, Error: err.Error()}, project.DefaultWatchdogPolicy())
		fmt.Fprintf(os.Stderr, "project tick: %v\n", err)
		os.Exit(1)
	}
	cmd := exec.CommandContext(context.Background(), os.Args[0], "master", manifestPath)
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
	runErr := cmd.Run()
	afterCtx, loadErr := store.LoadContext(fs.Arg(0))
	if runErr != nil {
		if loadErr == nil {
			_ = store.RecordAutoWakeOutcome(fs.Arg(0), decision.WakeID, time.Now().UTC(), project.WakeOutcome{Failed: true, Error: runErr.Error()}, project.DefaultWatchdogPolicy())
		}
		fmt.Fprintf(os.Stderr, "project tick: Yardmaster wake failed: %v\n", runErr)
		os.Exit(1)
	}
	if loadErr != nil {
		fmt.Fprintf(os.Stderr, "project tick: load wake result: %v\n", loadErr)
		os.Exit(1)
	}
	progress := afterCtx.Project.State == project.LifecycleAccepted || afterCtx.Project.State == project.LifecycleCancelled || before != afterCtx.ProgressFingerprint()
	if err := store.RecordAutoWakeOutcome(fs.Arg(0), decision.WakeID, time.Now().UTC(), project.WakeOutcome{Progress: progress}, project.DefaultWatchdogPolicy()); err != nil {
		fmt.Fprintf(os.Stderr, "project tick: record wake outcome: %v\n", err)
		os.Exit(1)
	}
	decision.Reason = "WAKE_COMPLETED"
	printWakeDecision(decision, *jsonOutput)
}

func printWakeDecision(decision project.WakeDecision, jsonOutput bool) {
	if jsonOutput {
		_ = json.NewEncoder(os.Stdout).Encode(decision)
		return
	}
	fmt.Printf("Project: %s\nAction: %s\nReason: %s\n", decision.ProjectID, decision.Action, decision.Reason)
	if decision.WakeID != "" {
		fmt.Printf("Wake: %s (generation %d)\n", decision.WakeID, decision.Generation)
	}
}

func findProjectMasterManifest(projectID, explicit string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", fmt.Errorf("manifest %s: %w", explicit, err)
		}
		return explicit, nil
	}
	paths, err := filepath.Glob(filepath.Join("gary", "master", "pending", "*.json"))
	if err != nil {
		return "", err
	}
	for _, path := range paths {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			continue
		}
		var candidate struct {
			ID        string `json:"id"`
			ProjectID string `json:"project_id"`
		}
		if json.Unmarshal(data, &candidate) != nil {
			continue
		}
		if candidate.ProjectID == projectID || (candidate.ProjectID == "" && candidate.ID == projectID) {
			return path, nil
		}
	}
	return "", errors.New("no pending master manifest found for project")
}
