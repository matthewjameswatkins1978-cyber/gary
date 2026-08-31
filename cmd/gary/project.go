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
	"sort"
	"strconv"
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
	if err == nil && mode == "inspect" && ctx.Contract.ContractVersion == 0 {
		if _, contractErr := project.DefaultStore().EnsureAcceptanceContract(fs.Arg(0)); contractErr != nil {
			err = contractErr
		} else {
			ctx, err = project.DefaultStore().LoadContext(fs.Arg(0))
		}
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
	manual, err := decodeProjectManual(text, input.Project.MasterPacketDigest)
	if err != nil {
		return project.ProjectManual{}, fmt.Errorf("project manual JSON: %w; output=%s", err, compactPlannerError(text, 3000))
	}
	return manual, nil
}

// decodeProjectManual accepts only known, semantically equivalent Yardmaster
// representations. The durable store remains the strict validation boundary.
func decodeProjectManual(text, packetDigest string) (project.ProjectManual, error) {
	dec := json.NewDecoder(strings.NewReader(text))
	var raw map[string]json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return project.ProjectManual{}, err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return project.ProjectManual{}, errors.New("project manual returned more than one JSON value")
	}
	if version, ok := raw["manual_version"]; ok {
		var asString string
		if json.Unmarshal(version, &asString) == nil {
			n, err := parseManualVersion(asString)
			if err != nil {
				return project.ProjectManual{}, fmt.Errorf("manual_version must be an integer: %w", err)
			}
			encoded, _ := json.Marshal(n)
			raw["manual_version"] = encoded
		}
	}
	for _, field := range []string{"objective", "definition_of_done"} {
		if value, ok := raw[field]; ok {
			if encoded, ok := normalizeTopLevelText(value); ok {
				raw[field] = encoded
			}
		}
	}
	for field, typ := range map[string]string{
		"hard_constraints": "hard_constraint", "architecture_decisions": "architecture_decision",
		"technology_choices": "technology_choice", "forbidden_changes": "forbidden_change",
		"functional_requirements": "requirement", "non_functional_requirements": "non_functional_requirement",
		"acceptance_criteria": "acceptance", "important_context": "context",
		"important_rationale": "rationale", "known_dependencies": "dependency",
		"known_external_services": "external_service", "known_risks": "risk",
		"open_questions": "open_question", "active_decisions": "active_decision",
		"superseded_decisions": "superseded_decision",
	} {
		if value, ok := raw[field]; ok {
			rawItems, err := normalizeArrayValue(value)
			if err != nil {
				return project.ProjectManual{}, fmt.Errorf("%s: %w", field, err)
			}
			items := make([]map[string]json.RawMessage, 0, len(rawItems))
			for i, rawItem := range rawItems {
				item, err := normalizeRequirementItem(rawItem, field, typ, i, packetDigest)
				if err != nil {
					return project.ProjectManual{}, fmt.Errorf("%s[%d]: %w", field, i, err)
				}
				if _, ok := item["text"]; !ok {
					for _, candidate := range []string{"description", "decision", "choice", "command", "expected_outcome", "purpose", "technology", "objective", "strategy", "capability", "requirement", "statement", "risk", "tool", "name", "title", "value", "reason", "goal", "criterion"} {
						if candidateValue, exists := item[candidate]; exists {
							item["text"] = candidateValue
							break
						}
					}
					if _, ok := item["text"]; !ok {
						keys := make([]string, 0, len(item))
						for key := range item {
							keys = append(keys, key)
						}
						sort.Strings(keys)
						for _, key := range keys {
							if key == "id" || key == "status" || key == "type" || key == "source" || key == "source_locator" || key == "locator" {
								continue
							}
							var candidate string
							if json.Unmarshal(item[key], &candidate) == nil && strings.TrimSpace(candidate) != "" {
								item["text"] = item[key]
								break
							}
						}
					}
				}
				if _, ok := item["type"]; !ok {
					encoded, _ := json.Marshal(typ)
					item["type"] = encoded
				}
				if _, ok := item["id"]; !ok {
					encoded, _ := json.Marshal(fmt.Sprintf("%s-%03d", field, i+1))
					item["id"] = encoded
				}
				normalizeSource(item, packetDigest, field)
				items = append(items, item)
			}
			encoded, _ := json.Marshal(items)
			raw[field] = encoded
		}
	}
	if value, ok := raw["conflicts"]; ok {
		var rawItems []json.RawMessage
		if err := json.Unmarshal(value, &rawItems); err != nil {
			return project.ProjectManual{}, fmt.Errorf("conflicts: %w", err)
		}
		items := make([]map[string]json.RawMessage, 0, len(rawItems))
		for i, rawItem := range rawItems {
			item, err := normalizeRequirementItem(rawItem, "conflicts", "conflict", i, packetDigest)
			if err != nil {
				return project.ProjectManual{}, fmt.Errorf("conflicts[%d]: %w", i, err)
			}
			normalizeSource(item, packetDigest, "conflicts")
			items = append(items, item)
		}
		encoded, _ := json.Marshal(items)
		raw["conflicts"] = encoded
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return project.ProjectManual{}, err
	}
	var manual project.ProjectManual
	if err := json.Unmarshal(encoded, &manual); err != nil {
		return project.ProjectManual{}, err
	}
	return manual, nil
}

func normalizeArrayValue(value json.RawMessage) ([]json.RawMessage, error) {
	var items []json.RawMessage
	if err := json.Unmarshal(value, &items); err == nil {
		return items, nil
	}
	var scalar string
	if err := json.Unmarshal(value, &scalar); err != nil {
		return nil, err
	}
	return []json.RawMessage{json.RawMessage(value)}, nil
}

func normalizeTopLevelText(value json.RawMessage) (json.RawMessage, bool) {
	var statements []json.RawMessage
	if json.Unmarshal(value, &statements) != nil {
		return nil, false
	}
	texts := make([]string, 0, len(statements))
	for _, statement := range statements {
		var asString string
		if json.Unmarshal(statement, &asString) == nil {
			texts = append(texts, asString)
			continue
		}
		var item map[string]json.RawMessage
		if json.Unmarshal(statement, &item) != nil {
			return nil, false
		}
		for _, field := range []string{"text", "description", "statement", "requirement", "criterion", "done"} {
			if candidate, ok := item[field]; ok {
				if json.Unmarshal(candidate, &asString) == nil {
					texts = append(texts, asString)
					break
				}
			}
		}
	}
	encoded, _ := json.Marshal(strings.Join(texts, "\n"))
	return encoded, true
}

func parseManualVersion(value string) (int, error) {
	if n, err := strconv.Atoi(value); err == nil && n > 0 {
		return n, nil
	}
	parts := strings.Split(value, ".")
	if len(parts) < 2 {
		return 0, fmt.Errorf("%q is not an integer version", value)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil || major < 1 {
		return 0, fmt.Errorf("%q is not an integer version", value)
	}
	for _, part := range parts[1:] {
		minor, err := strconv.Atoi(part)
		if err != nil || minor != 0 {
			return 0, fmt.Errorf("%q is not an integer version", value)
		}
	}
	return major, nil
}

func normalizeRequirementItem(raw json.RawMessage, field, typ string, index int, packetDigest string) (map[string]json.RawMessage, error) {
	var item map[string]json.RawMessage
	if err := json.Unmarshal(raw, &item); err == nil && item != nil {
		return item, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return nil, err
	}
	item = map[string]json.RawMessage{}
	for key, value := range map[string]string{"id": fmt.Sprintf("%s-%03d", field, index+1), "text": text, "type": typ} {
		encoded, _ := json.Marshal(value)
		item[key] = encoded
	}
	return item, nil
}

func normalizeSource(item map[string]json.RawMessage, packetDigest, field string) {
	if source, ok := item["source"]; ok {
		var sourceObject map[string]json.RawMessage
		if json.Unmarshal(source, &sourceObject) != nil {
			var locator string
			if json.Unmarshal(source, &locator) == nil {
				sourceObject = map[string]json.RawMessage{}
				encoded, _ := json.Marshal(locator)
				sourceObject["locator"] = encoded
			}
		}
		if sourceObject != nil {
			if _, ok := sourceObject["packet_digest"]; !ok {
				encoded, _ := json.Marshal(packetDigest)
				sourceObject["packet_digest"] = encoded
			}
			if _, ok := sourceObject["locator"]; !ok {
				encoded, _ := json.Marshal("MASTER_PACKET.md#" + field)
				sourceObject["locator"] = encoded
			}
			encoded, _ := json.Marshal(sourceObject)
			item["source"] = encoded
		}
		return
	}
	var source struct {
		PacketDigest string `json:"packet_digest"`
		Locator      string `json:"locator"`
	}
	locator, ok := item["source_locator"]
	if !ok {
		locator, ok = item["locator"]
	}
	if ok {
		_ = json.Unmarshal(locator, &source.Locator)
	}
	source.PacketDigest = packetDigest
	if source.Locator == "" {
		source.Locator = "MASTER_PACKET.md#" + field
	}
	encoded, _ := json.Marshal(source)
	item["source"] = encoded
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
