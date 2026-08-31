package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/config"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/project"
)

func cmdProject() {
	if len(os.Args) < 3 {
		printProjectUsage()
		os.Exit(1)
	}
	switch os.Args[2] {
	case "start":
		cmdProjectStart()
	case "status", "resume", "inspect":
		cmdProjectRead(os.Args[2])
	case "amend":
		cmdProjectAmend()
	default:
		printProjectUsage()
		os.Exit(1)
	}
}

func printProjectUsage() {
	fmt.Fprintln(os.Stderr, "Usage: gary project <start|status|resume|amend|inspect>")
	fmt.Fprintln(os.Stderr, "  gary project start [--id PROJECT-ID] [--repo URL] [--base-branch BRANCH] PACKET")
	fmt.Fprintln(os.Stderr, "  gary project status PROJECT-ID")
	fmt.Fprintln(os.Stderr, "  gary project resume PROJECT-ID")
	fmt.Fprintln(os.Stderr, "  gary project amend PROJECT-ID AMENDMENT")
	fmt.Fprintln(os.Stderr, "  gary project inspect PROJECT-ID")
}

func cmdProjectStart() {
	fs := flag.NewFlagSet("project start", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "stable project id")
	repo := fs.String("repo", "", "optional target repository")
	baseBranch := fs.String("base-branch", "", "optional target base branch")
	jsonOutput := fs.Bool("json", false, "machine-readable output")
	if err := fs.Parse(os.Args[3:]); err != nil || fs.NArg() != 1 {
		printProjectUsage()
		os.Exit(1)
	}
	packetPath := fs.Arg(0)
	packet, err := project.ReadPacketFile(packetPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "project start: %v\n", err)
		os.Exit(1)
	}
	projectID := *id
	if projectID == "" {
		projectID = projectIDFromPath(packetPath)
	}
	if !project.IsValidID(projectID) {
		fmt.Fprintf(os.Stderr, "project start: invalid project id %q\n", projectID)
		os.Exit(1)
	}
	name := projectNameFromPath(packetPath)
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "project start config: %v\n", err)
		os.Exit(1)
	}
	store := project.DefaultStore()
	ingestCtx, cancel := context.WithTimeout(context.Background(), yardmasterPlannerTimeout)
	defer cancel()
	projectCtx, err := store.CreateAndIngest(ingestCtx, projectID, name, *repo, *baseBranch, packet, func(ctx context.Context, input project.ProjectContext) (project.ProjectManual, error) {
		return generateProjectManual(ctx, cfg, input)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "project start: %v\n", err)
		fmt.Fprintf(os.Stderr, "project packet remains at %s\n", filepath.Join(store.Root(), projectID, "source", "MASTER_PACKET.md"))
		os.Exit(1)
	}
	printProjectContext(projectCtx, *jsonOutput)
}

func cmdProjectRead(mode string) {
	fs := flag.NewFlagSet("project "+mode, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	jsonOutput := fs.Bool("json", false, "machine-readable output")
	if err := fs.Parse(os.Args[3:]); err != nil || fs.NArg() != 1 {
		printProjectUsage()
		os.Exit(1)
	}
	var ctx project.ProjectContext
	var err error
	if mode == "resume" {
		ctx, err = project.DefaultStore().ResumeProject(fs.Arg(0))
	} else {
		ctx, err = project.DefaultStore().LoadContext(fs.Arg(0))
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "project %s: %v\n", mode, err)
		os.Exit(1)
	}
	printProjectContext(ctx, *jsonOutput)
}

func cmdProjectAmend() {
	fs := flag.NewFlagSet("project amend", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	jsonOutput := fs.Bool("json", false, "machine-readable output")
	if err := fs.Parse(os.Args[3:]); err != nil || fs.NArg() != 2 {
		printProjectUsage()
		os.Exit(1)
	}
	contents, err := project.ReadPacketFile(fs.Arg(1))
	if err != nil {
		fmt.Fprintf(os.Stderr, "project amend: %v\n", err)
		os.Exit(1)
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "project amend config: %v\n", err)
		os.Exit(1)
	}
	ingestCtx, cancel := context.WithTimeout(context.Background(), yardmasterPlannerTimeout)
	defer cancel()
	projectCtx, err := project.DefaultStore().AmendAndIngest(ingestCtx, fs.Arg(0), contents, func(ctx context.Context, input project.ProjectContext) (project.ProjectManual, error) {
		return generateProjectManual(ctx, cfg, input)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "project amend: %v\n", err)
		os.Exit(1)
	}
	printProjectContext(projectCtx, *jsonOutput)
}

func printProjectContext(ctx project.ProjectContext, jsonOutput bool) {
	if jsonOutput {
		_ = json.NewEncoder(os.Stdout).Encode(struct {
			Project project.Project       `json:"project"`
			State   project.ProjectState  `json:"state"`
			Manual  project.ProjectManual `json:"manual"`
		}{ctx.Project, ctx.State, ctx.Manual})
		return
	}
	fmt.Printf("Project: %s\nState: %s\nMaster packet: %s\nManual: v%d\nPhase: %s\n", ctx.Project.ProjectID, ctx.Project.State, ctx.Project.MasterPacketDigest, ctx.Project.ProjectManualVersion, ctx.State.CurrentPhase)
	fmt.Printf("Objective: %s\n", ctx.Manual.Objective)
	fmt.Printf("Source: %s\n", filepath.Join(project.DefaultStore().Root(), ctx.Project.ProjectID, "source", "MASTER_PACKET.md"))
}

func projectIDFromPath(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	base = strings.ToLower(base)
	base = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(base, "-")
	base = strings.Trim(base, "-")
	if base == "" {
		return "project"
	}
	if len(base) > 60 {
		base = base[:60]
	}
	return base
}

func projectNameFromPath(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return strings.ReplaceAll(base, "-", " ")
}

func generateProjectManual(ctx context.Context, cfg *config.Config, input project.ProjectContext) (project.ProjectManual, error) {
	if cfg.GCPProject == "" {
		return project.ProjectManual{}, errors.New("GARY_GCP_PROJECT is required for Yardmaster project ingestion")
	}
	prompt := buildProjectManualPrompt(input)
	text, err := callProjectYardmasterJSON(ctx, cfg, prompt)
	if err != nil {
		return project.ProjectManual{}, err
	}
	var manual project.ProjectManual
	dec := json.NewDecoder(strings.NewReader(text))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&manual); err != nil {
		return project.ProjectManual{}, fmt.Errorf("project manual JSON: %w; output=%s", err, compactPlannerError(text, 3000))
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return project.ProjectManual{}, errors.New("project manual returned more than one JSON value")
	}
	return manual, nil
}

func buildProjectManualPrompt(input project.ProjectContext) string {
	var amendments strings.Builder
	for _, amendment := range input.Amendments {
		fmt.Fprintf(&amendments, "\n--- AMENDMENT %s ---\n%s\n", amendment.ID, amendment.Contents)
	}
	return fmt.Sprintf(`You are the GARY Yardmaster performing durable project intake.

Read the complete master packet below. Distinguish requirements from suggestions
and background. Preserve ambiguity and contradictions instead of inventing
answers. Record important requirements with stable IDs and source locators into
the supplied packet digest. Return only one JSON object matching the
gary.project-manual.v1 structure. Use status values KNOWN, INFERRED, UNRESOLVED,
or CONTRADICTORY where useful. Keep the manual operational and compact, not a
generic prose summary.

PROJECT ID: %s
PACKET DIGEST: %s

MASTER PACKET
=============
%s
%s

Required top-level fields include schema, project_id, objective,
definition_of_done, hard_constraints, architecture_decisions,
technology_choices, forbidden_changes, functional_requirements,
non_functional_requirements, acceptance_criteria, important_context,
important_rationale, known_dependencies, known_external_services, known_risks,
open_questions, conflicts, current_phase, current_strategy,
completed_capabilities, remaining_capabilities, active_decisions,
superseded_decisions, source_packet_digest, manual_version and manual_digest.
`, input.Project.ProjectID, input.Project.MasterPacketDigest, input.Packet, amendments.String())
}

func callProjectYardmasterJSON(ctx context.Context, cfg *config.Config, prompt string) (string, error) {
	token, err := vertexAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("project Yardmaster auth: %w", err)
	}
	body, err := json.Marshal(yardmasterRequestBody(prompt))
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, yardmasterVertexEndpoint(cfg.GCPProject, cfg.MasterModel), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("project Yardmaster transport: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("project Yardmaster API %d: %s", resp.StatusCode, compactPlannerError(string(respBody), 2000))
	}
	var apiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return "", err
	}
	if len(apiResp.Candidates) == 0 {
		return "", errors.New("project Yardmaster returned no candidates")
	}
	var text strings.Builder
	for _, part := range apiResp.Candidates[0].Content.Parts {
		text.WriteString(part.Text)
	}
	return stripJSONFence(strings.TrimSpace(text.String())), nil
}
