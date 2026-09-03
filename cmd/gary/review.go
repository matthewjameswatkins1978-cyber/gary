package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/project"
)

type projectExplanation struct {
	Schema          string                   `json:"schema"`
	ReadOnly        bool                     `json:"read_only"`
	GeneratedAt     string                   `json:"generated_at"`
	Project         project.Project          `json:"project"`
	State           project.ProjectState     `json:"state"`
	Packet          reviewPacketEvidence     `json:"packet"`
	Manual          reviewManualEvidence     `json:"manual"`
	Amendments      []project.Amendment      `json:"amendments,omitempty"`
	Feedback        []project.FeedbackRecord `json:"feedback,omitempty"`
	MasterRuns      []masterStatus           `json:"master_runs,omitempty"`
	EvidenceSources []string                 `json:"evidence_sources"`
	Basis           []string                 `json:"basis"`
}

type reviewPacketEvidence struct {
	Version int    `json:"version"`
	Digest  string `json:"digest"`
	Source  string `json:"source"`
}

type reviewManualEvidence struct {
	Version        int    `json:"version"`
	Digest         string `json:"digest"`
	Objective      string `json:"objective"`
	DefinitionDone string `json:"definition_of_done"`
	Source         string `json:"source"`
}

func cmdProjectFeedback() {
	fs := flag.NewFlagSet("project feedback", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	scope := fs.String("scope", "", "note, project_preference, project_correction, or propose_doctrine")
	text := fs.String("text", "", "explicit feedback text")
	actor := fs.String("actor", os.Getenv("GARY_REVIEW_ACTOR"), "recorded actor (defaults to GARY_REVIEW_ACTOR or operator)")
	decision := fs.String("decision", "", "optional referenced decision")
	child := fs.String("child-task-id", "", "optional referenced child task")
	jsonOutput := fs.Bool("json", false, "machine-readable output")
	args, err := normalizeReviewArgs(os.Args[3:], map[string]bool{"--scope": true, "--text": true, "--actor": true, "--decision": true, "--child-task-id": true})
	if err != nil || fs.Parse(args) != nil || fs.NArg() != 1 || strings.TrimSpace(*scope) == "" || strings.TrimSpace(*text) == "" {
		printProjectUsage()
		os.Exit(1)
	}
	if strings.TrimSpace(*actor) == "" {
		*actor = "operator"
	}
	record, err := project.DefaultStore().AppendFeedback(fs.Arg(0), *actor, *scope, *decision, *child, *text)
	if err != nil {
		fmt.Fprintf(os.Stderr, "project feedback: %v\n", err)
		os.Exit(1)
	}
	if *jsonOutput {
		_ = json.NewEncoder(os.Stdout).Encode(record)
		return
	}
	fmt.Printf("Feedback: %s\nProject: %s\nScope: %s\nStatus: %s\n", record.FeedbackID, record.ProjectID, record.Scope, record.Status)
	if record.Status == project.FeedbackPending {
		fmt.Println("Effect: pending until GARY reaches the next PLANNING boundary")
	} else {
		fmt.Println("Effect: recorded only; no execution or doctrine change")
	}
}

func cmdProjectExplain() {
	fs := flag.NewFlagSet("project explain", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	masterID := fs.String("master-id", "", "optional master decision record")
	childTaskID := fs.String("child-task-id", "", "optional child task result")
	wave := fs.Int("wave", 0, "optional master wave number")
	jsonOutput := fs.Bool("json", false, "machine-readable output")
	args, err := normalizeReviewArgs(os.Args[3:], map[string]bool{"--master-id": true, "--child-task-id": true, "--wave": true})
	if err != nil || fs.Parse(args) != nil || fs.NArg() != 1 || *wave < 0 {
		printProjectUsage()
		os.Exit(1)
	}
	explanation, err := buildProjectExplanation(fs.Arg(0), *masterID, *childTaskID, *wave)
	if err != nil {
		fmt.Fprintf(os.Stderr, "project explain: %v\n", err)
		os.Exit(1)
	}
	if *jsonOutput {
		_ = json.NewEncoder(os.Stdout).Encode(explanation)
		return
	}
	printProjectExplanation(explanation, *childTaskID, *wave)
}

// normalizeReviewArgs keeps the CLI friendly when a project ID is placed
// before flags, matching the existing `project status ID` convention while
// still using Go's strict FlagSet parsing underneath.
func normalizeReviewArgs(args []string, valueFlags map[string]bool) ([]string, error) {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			positional = append(positional, arg)
			continue
		}
		flags = append(flags, arg)
		name := strings.SplitN(arg, "=", 2)[0]
		if valueFlags[name] && !strings.Contains(arg, "=") {
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return nil, fmt.Errorf("flag %s requires a value", name)
			}
			flags = append(flags, args[i+1])
			i++
		}
	}
	if len(positional) != 1 {
		return nil, errors.New("exactly one project id is required")
	}
	return append(flags, positional[0]), nil
}

func buildProjectExplanation(projectID, masterID, childTaskID string, wave int) (projectExplanation, error) {
	ctx, err := project.DefaultStore().LoadContext(projectID)
	if err != nil {
		return projectExplanation{}, err
	}
	explanation := projectExplanation{
		Schema: "gary.project-explanation.v1", ReadOnly: true, GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Project: ctx.Project, State: ctx.State,
		Packet:     reviewPacketEvidence{Version: ctx.Project.MasterPacketVersion, Digest: ctx.Project.MasterPacketDigest, Source: filepath.Join(project.DefaultStore().Root(), projectID, "source", "MASTER_PACKET.md")},
		Manual:     reviewManualEvidence{Version: ctx.Project.ProjectManualVersion, Digest: ctx.Project.ManualDigest, Objective: ctx.Manual.Objective, DefinitionDone: ctx.Manual.DefinitionOfDone, Source: filepath.Join(project.DefaultStore().Root(), projectID, "manuals", fmt.Sprintf("project-manual-v%03d.json", ctx.Project.ProjectManualVersion))},
		Amendments: ctx.Amendments, Feedback: ctx.Feedback,
		Basis: []string{"durable project.json and state.json", "immutable master packet digest", "current versioned manual digest", "append-only review events", "recorded master wave plans and worker results", "GARY rulebook and project-context operating boundary"},
	}
	explanation.EvidenceSources = append(explanation.EvidenceSources,
		filepath.Join(project.DefaultStore().Root(), projectID, "project.json"),
		filepath.Join(project.DefaultStore().Root(), projectID, "state.json"),
		explanation.Packet.Source, explanation.Manual.Source,
		filepath.Join(project.DefaultStore().Root(), projectID, "feedback", "events.jsonl"),
		"GARY_RULEBOOK.md", "docs/project-context.md")
	for _, amendment := range ctx.Amendments {
		explanation.EvidenceSources = append(explanation.EvidenceSources, filepath.Join(project.DefaultStore().Root(), projectID, filepath.FromSlash(amendment.Path)))
	}

	statuses, err := loadProjectMasterStatuses(projectID, masterID)
	if err != nil {
		return projectExplanation{}, err
	}
	for i := range statuses {
		filterMasterStatus(&statuses[i], childTaskID, wave)
		if (childTaskID != "" || wave != 0) && len(statuses[i].Waves) == 0 {
			continue
		}
		if masterID != "" && statuses[i].ID != masterID {
			continue
		}
		explanation.MasterRuns = append(explanation.MasterRuns, statuses[i])
		explanation.EvidenceSources = append(explanation.EvidenceSources, filepath.Join("gary", "master", "status", statuses[i].ID+".json"))
		for _, masterWave := range statuses[i].Waves {
			for _, result := range masterWave.Results {
				resultPath := filepath.Join("results", "runs", result.JobID+".json")
				if _, err := os.Stat(resultPath); err == nil {
					explanation.EvidenceSources = append(explanation.EvidenceSources, resultPath)
				}
			}
		}
	}
	if masterID != "" && len(explanation.MasterRuns) == 0 {
		return projectExplanation{}, errors.New("requested master decision was not found for project")
	}
	if childTaskID != "" && !explanationHasChild(explanation.MasterRuns) {
		return projectExplanation{}, fmt.Errorf("requested child task %q was not found for project", childTaskID)
	}
	return explanation, nil
}

func loadProjectMasterStatuses(projectID, masterID string) ([]masterStatus, error) {
	pattern := filepath.Join("gary", "master", "status", "*.json")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var statuses []masterStatus
	for _, path := range paths {
		if masterID != "" && filepath.Base(path) != masterID+".json" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var status masterStatus
		if err := json.Unmarshal(data, &status); err != nil {
			return nil, fmt.Errorf("read master status %s: %w", path, err)
		}
		if status.ProjectID != projectID && !(status.ProjectID == "" && status.ID == projectID) {
			continue
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func filterMasterStatus(status *masterStatus, childTaskID string, wave int) {
	if childTaskID == "" && wave == 0 {
		return
	}
	filtered := status.Waves[:0]
	for _, masterWave := range status.Waves {
		if wave != 0 && masterWave.Wave != wave {
			continue
		}
		if childTaskID != "" {
			results := masterWave.Results[:0]
			for _, result := range masterWave.Results {
				if result.JobID == childTaskID {
					results = append(results, result)
				}
			}
			masterWave.Results = results
			if len(results) == 0 {
				continue
			}
		}
		filtered = append(filtered, masterWave)
	}
	status.Waves = filtered
}

func explanationHasChild(statuses []masterStatus) bool {
	for _, status := range statuses {
		for _, wave := range status.Waves {
			if len(wave.Results) > 0 {
				return true
			}
		}
	}
	return false
}

func printProjectExplanation(explanation projectExplanation, childTaskID string, wave int) {
	fmt.Printf("GARY // PROJECT EXPLAIN (READ ONLY)\nProject: %s\nState: %s\nPacket: v%d %s\nManual: v%d %s\n", explanation.Project.ProjectID, explanation.Project.State, explanation.Packet.Version, explanation.Packet.Digest, explanation.Manual.Version, explanation.Manual.Digest)
	fmt.Printf("Phase: %s\nObjective: %s\nFeedback records: %d\n", explanation.State.CurrentPhase, explanation.Manual.Objective, len(explanation.Feedback))
	if len(explanation.MasterRuns) == 0 {
		fmt.Println("Recorded master decisions: none")
	}
	for _, status := range explanation.MasterRuns {
		fmt.Printf("Master %s: state=%s branch=%s summary=%s\n", status.ID, status.State, status.StagingBranch, status.Summary)
		for _, masterWave := range status.Waves {
			fmt.Printf("  Wave %d: mode=%s plan=%s\n", masterWave.Wave, masterWave.PlannerMode, masterWave.PlanSummary)
			for _, result := range masterWave.Results {
				costUSD := result.TotalCostEstimated
				if result.Cost != nil {
					costUSD = result.Cost.TotalCostUSD
				}
				fmt.Printf("    Child %s: status=%s accepted=%t attempts=%d branch=%s commit=%s cost=$%.4f\n", result.JobID, result.FinalStatus, result.Accepted, result.Attempts, result.Branch, result.CommitSHA, costUSD)
			}
		}
	}
	if childTaskID != "" {
		fmt.Printf("Child filter: %s\n", childTaskID)
	}
	if wave != 0 {
		fmt.Printf("Wave filter: %d\n", wave)
	}
	fmt.Println("Basis: recorded plans, worker results, project state, packet/manual provenance, and review events; no hidden reasoning is claimed.")
}
