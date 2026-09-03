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
	"time"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/config"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/doctrine"
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
	case "board":
		cmdProjectBoard()
	case "tick":
		cmdProjectTick()
	case "amend":
		cmdProjectAmend()
	case "explain":
		cmdProjectExplain()
	case "feedback":
		cmdProjectFeedback()
	default:
		printProjectUsage()
		os.Exit(1)
	}
}

func printProjectUsage() {
	fmt.Fprintln(os.Stderr, "Usage: gary project <start|status|resume|amend|board|inspect|explain|feedback>")
	fmt.Fprintln(os.Stderr, "  gary project start [--id PROJECT-ID] [--repo URL] [--base-branch BRANCH] PACKET")
	fmt.Fprintln(os.Stderr, "  gary project status PROJECT-ID")
	fmt.Fprintln(os.Stderr, "  gary project resume PROJECT-ID")
	fmt.Fprintln(os.Stderr, "  gary project board [--json]")
	fmt.Fprintln(os.Stderr, "  gary project tick PROJECT-ID [--manifest PATH] [--json]")
	fmt.Fprintln(os.Stderr, "  gary project amend PROJECT-ID AMENDMENT")
	fmt.Fprintln(os.Stderr, "  gary project inspect PROJECT-ID")
	fmt.Fprintln(os.Stderr, "  gary project explain PROJECT-ID [--master-id ID] [--child-task-id ID] [--wave N]")
	fmt.Fprintln(os.Stderr, "  gary project feedback PROJECT-ID --scope SCOPE --text TEXT [--actor ACTOR]")
	fmt.Fprintln(os.Stderr, "    scopes: note, project_preference, project_correction, propose_doctrine")
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
	board, _ := project.DefaultStore().LoadBoard()
	var boardEntry *project.BoardEntry
	for i := range board.Projects {
		if board.Projects[i].ProjectID == ctx.Project.ProjectID {
			boardEntry = &board.Projects[i]
			break
		}
	}
	if jsonOutput {
		_ = json.NewEncoder(os.Stdout).Encode(struct {
			Project  project.Project          `json:"project"`
			State    project.ProjectState     `json:"state"`
			Manual   project.ProjectManual    `json:"manual"`
			Feedback []project.FeedbackRecord `json:"feedback,omitempty"`
			Board    *project.BoardEntry      `json:"board,omitempty"`
		}{ctx.Project, ctx.State, ctx.Manual, ctx.Feedback, boardEntry})
		return
	}
	fmt.Printf("Project: %s\nState: %s\nMaster packet: %s\nManual: v%d\nPhase: %s\n", ctx.Project.ProjectID, ctx.Project.State, ctx.Project.MasterPacketDigest, ctx.Project.ProjectManualVersion, ctx.State.CurrentPhase)
	if boardEntry != nil {
		fmt.Printf("Board: %s\n", boardEntry.Status)
	}
	fmt.Printf("Objective: %s\n", ctx.Manual.Objective)
	if len(ctx.Feedback) > 0 {
		pending := 0
		for _, feedback := range ctx.Feedback {
			if feedback.Status == project.FeedbackPending {
				pending++
			}
		}
		fmt.Printf("Conduct review: %d record(s), %d pending safe-boundary application(s)\n", len(ctx.Feedback), pending)
	}
	fmt.Printf("Source: %s\n", filepath.Join(project.DefaultStore().Root(), ctx.Project.ProjectID, "source", "MASTER_PACKET.md"))
}

func cmdProjectBoard() {
	fs := flag.NewFlagSet("project board", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	jsonOutput := fs.Bool("json", false, "machine-readable output")
	if err := fs.Parse(os.Args[3:]); err != nil || fs.NArg() != 0 {
		printProjectUsage()
		os.Exit(1)
	}
	board, err := project.DefaultStore().LoadBoard()
	if err != nil {
		fmt.Fprintf(os.Stderr, "project board: %v\n", err)
		os.Exit(1)
	}
	if *jsonOutput {
		_ = json.NewEncoder(os.Stdout).Encode(board)
		return
	}
	fmt.Printf("GARY // PROJECT BOARD\nUpdated: %s\n", board.UpdatedAt)
	for _, entry := range board.Projects {
		fmt.Printf("%-20s %-12s %s\n", entry.ProjectID, entry.Status, entry.ProjectName)
	}
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
	// The durable store owns manual revision numbering. Yardmaster output may
	// confuse the product version (for example "0.1.0") with this internal
	// revision, so normalize any supplied value to a harmless placeholder.
	// CreateAndIngest/AmendAndIngest replace it with the authoritative version
	// before validation and persistence.
	if _, ok := raw["manual_version"]; ok {
		encoded, _ := json.Marshal(1)
		raw["manual_version"] = encoded
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
				textValue, hasText := item["text"]
				if !hasText || !nonEmptyJSONString(textValue) {
					delete(item, "text")
					for _, candidate := range []string{"description", "decision", "choice", "command", "expected_outcome", "purpose", "technology", "objective", "strategy", "capability", "requirement", "statement", "risk", "tool", "name", "title", "value", "reason", "goal", "criterion"} {
						if candidateValue, exists := item[candidate]; exists && nonEmptyJSONString(candidateValue) {
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
				idValue, hasID := item["id"]
				if !hasID || !nonEmptyJSONString(idValue) {
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
	for _, field := range []string{"completed_capabilities", "remaining_capabilities"} {
		if value, ok := raw[field]; ok {
			encoded, err := normalizeCapabilityList(value)
			if err != nil {
				return project.ProjectManual{}, fmt.Errorf("%s: %w", field, err)
			}
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
	// Board lifecycle is a runtime invariant. Treat model output as a hint for
	// the optional completion wording, never as authority over the statuses or
	// the Yardmaster-owned completion boundary.
	manual.BoardPolicy = project.NormalizeBoardPolicy(manual.BoardPolicy)
	return manual, nil
}

func nonEmptyJSONString(value json.RawMessage) bool {
	var text string
	return json.Unmarshal(value, &text) == nil && strings.TrimSpace(text) != ""
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

func normalizeCapabilityList(value json.RawMessage) (json.RawMessage, error) {
	items, err := normalizeArrayValue(value)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(items))
	for _, rawItem := range items {
		var text string
		if json.Unmarshal(rawItem, &text) == nil {
			if strings.TrimSpace(text) != "" {
				out = append(out, strings.TrimSpace(text))
			}
			continue
		}
		var item map[string]json.RawMessage
		if err := json.Unmarshal(rawItem, &item); err != nil {
			return nil, err
		}
		var title, description string
		for _, key := range []string{"title", "name", "capability", "id"} {
			if value, ok := item[key]; ok && json.Unmarshal(value, &title) == nil && strings.TrimSpace(title) != "" {
				break
			}
			title = ""
		}
		for _, key := range []string{"description", "text", "objective", "purpose"} {
			if value, ok := item[key]; ok && json.Unmarshal(value, &description) == nil && strings.TrimSpace(description) != "" {
				break
			}
			description = ""
		}
		switch {
		case strings.TrimSpace(title) != "" && strings.TrimSpace(description) != "":
			out = append(out, strings.TrimSpace(title)+": "+strings.TrimSpace(description))
		case strings.TrimSpace(description) != "":
			out = append(out, strings.TrimSpace(description))
		case strings.TrimSpace(title) != "":
			out = append(out, strings.TrimSpace(title))
		default:
			return nil, errors.New("capability item has no usable text")
		}
	}
	encoded, _ := json.Marshal(out)
	return encoded, nil
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
			packetValue, hasPacket := sourceObject["packet_digest"]
			if !hasPacket || !nonEmptyJSONString(packetValue) {
				encoded, _ := json.Marshal(packetDigest)
				sourceObject["packet_digest"] = encoded
			}
			locatorValue, hasLocator := sourceObject["locator"]
			if !hasLocator || !nonEmptyJSONString(locatorValue) {
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

COMPLETION PRINCIPLE
====================
%s

Read the complete master packet below. Distinguish requirements from suggestions
and background. Preserve ambiguity and contradictions instead of inventing
answers. Record important requirements with stable IDs and source locators into
the supplied packet digest. Return only one JSON object matching the
gary.project-manual.v1 structure. Use status values KNOWN, INFERRED, UNRESOLVED,
or CONTRADICTORY where useful.

The immutable master packet is persisted separately and remains the source of
truth. The manual is an operational index for planning and narrow child
contexts, NOT a restatement of the packet. Preserve all safety-critical,
acceptance-critical, architectural, and scope-boundary semantics, but merge
closely related details where that does not weaken meaning. Keep the entire
manual compact enough to finish as valid JSON. Prefer concise requirement text,
avoid prose duplication across categories, keep capability lists short, and
emit completed_capabilities and remaining_capabilities as arrays of strings.
Every requirement-like item must have usable non-empty text; omit genuinely
empty items rather than emitting placeholders.

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
Also emit board_policy with IN_PROCESS, COMPLETED, Yardmaster, and a concise
completion condition. The Yardmaster alone may move the board entry to
COMPLETED, and only after project-level acceptance; pauses and worker failures
remain IN_PROCESS.
`, doctrine.CompletionPrinciple, input.Project.ProjectID, input.Project.MasterPacketDigest, input.Packet, amendments.String())
}

func callProjectYardmasterJSON(ctx context.Context, cfg *config.Config, prompt string) (string, error) {
	token, err := vertexAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("project Yardmaster auth: %w", err)
	}
	requestBody := yardmasterRequestBody(prompt)
	if generationConfig, ok := requestBody["generationConfig"].(map[string]interface{}); ok {
		// Durable intake can legitimately need more output than a one-wave plan.
		// Keep the planner's normal 8k budget unchanged while allowing the one-time
		// project manual to finish as valid JSON for large master packets.
		generationConfig["maxOutputTokens"] = 32768
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	const maxAttempts = 4
	for attempt := 1; attempt <= maxAttempts; attempt++ {
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
		respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if readErr != nil {
			return "", readErr
		}
		if resp.StatusCode == http.StatusTooManyRequests && attempt < maxAttempts {
			delay := time.Duration(attempt*attempt*5) * time.Second
			fmt.Fprintf(os.Stderr, "project Yardmaster API 429; retrying attempt %d/%d after %s\n", attempt+1, maxAttempts, delay)
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return "", ctx.Err()
			case <-timer.C:
			}
			continue
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
	return "", errors.New("project Yardmaster retry budget exhausted")
}
