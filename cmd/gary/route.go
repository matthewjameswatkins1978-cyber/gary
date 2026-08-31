package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/config"
)

const telltailAdapterProtocol = "telltail.adapter.v1"

type routeRequest struct {
	Protocol  string         `json:"protocol"`
	CaseID    string         `json:"case_id"`
	ProbeKind string         `json:"probe_kind"`
	Situation map[string]any `json:"situation"`
	Choices   []routeChoice  `json:"choices"`
}

type routeChoice struct {
	ID         string  `json:"id"`
	CostScore  float64 `json:"cost_score"`
	Eligible   bool    `json:"eligible"`
	Sufficient bool    `json:"sufficient"`
	Priority   *int    `json:"priority,omitempty"`
}

type routeDecision struct {
	Schema      string   `json:"schema"`
	CaseID      string   `json:"case_id"`
	Action      string   `json:"action"`
	ChoiceID    string   `json:"choice_id"`
	RouteID     string   `json:"route_id"`
	Provider    string   `json:"provider"`
	Model       string   `json:"model"`
	ReasonCodes []string `json:"reason_codes,omitempty"`
}

func cmdRoute() {
	fs := flag.NewFlagSet("route", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "decide without launching a worker")
	jsonOutput := fs.Bool("json", false, "use the machine-readable protocol")
	if err := fs.Parse(os.Args[2:]); err != nil {
		os.Exit(2)
	}
	if !*dryRun || !*jsonOutput || fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: gary route --dry-run --json")
		os.Exit(2)
	}

	var req routeRequest
	dec := json.NewDecoder(io.LimitReader(os.Stdin, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		fmt.Fprintf(os.Stderr, "route input: %v\n", err)
		os.Exit(2)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			fmt.Fprintln(os.Stderr, "route input: expected one JSON object")
		} else {
			fmt.Fprintf(os.Stderr, "route input: %v\n", err)
		}
		os.Exit(2)
	}

	decision, err := decideRoute(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "route decision: %v\n", err)
		os.Exit(2)
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "route config: %v\n", err)
		os.Exit(2)
	}
	provider := "google-cloud-batch"
	if cfg.WorkerRuntime == "managed" {
		provider = "google-managed-agent"
	}
	if requested := stringValue(req.Situation, "requested_provider"); requested != "" && requested != provider {
		fmt.Fprintf(os.Stderr, "route provider mismatch: requested %q, configured %q\n", requested, provider)
		os.Exit(2)
	}
	if requested := stringValue(req.Situation, "requested_model"); requested != "" && requested != cfg.ImplementerModel {
		fmt.Fprintf(os.Stderr, "route model mismatch: requested %q, configured %q\n", requested, cfg.ImplementerModel)
		os.Exit(2)
	}
	decision.RouteID = decision.ChoiceID
	decision.Provider = provider
	decision.Model = cfg.ImplementerModel
	if err := json.NewEncoder(os.Stdout).Encode(decision); err != nil {
		fmt.Fprintf(os.Stderr, "route output: %v\n", err)
		os.Exit(1)
	}
}

func decideRoute(req routeRequest) (routeDecision, error) {
	if req.Protocol != telltailAdapterProtocol {
		return routeDecision{}, fmt.Errorf("unsupported protocol %q", req.Protocol)
	}
	if req.CaseID == "" || req.ProbeKind != "route" {
		return routeDecision{}, errors.New("case_id and probe_kind=route are required")
	}
	if len(req.Choices) == 0 {
		return routeDecision{}, errors.New("at least one choice is required")
	}
	seen := map[string]bool{}
	for _, c := range req.Choices {
		if c.ID == "" {
			return routeDecision{}, errors.New("choice id is required")
		}
		if seen[c.ID] {
			return routeDecision{}, fmt.Errorf("duplicate choice %q", c.ID)
		}
		seen[c.ID] = true
	}

	// These are GARY's safety decisions: authority and frozen constraints take
	// precedence over cost, decomposition precedes worker selection, and a
	// bounded repair keeps useful progress when the evidence supports it.
	if boolValue(req.Situation, "reversible") == false &&
		stringValue(req.Situation, "external_impact") == "high" &&
		boolValue(req.Situation, "worker_authority") == false {
		return namedDecision(req, "interrupt", "interrupt", "outside_authority", "irreversible")
	}
	if boolValue(req.Situation, "candidate_violates") && len(arrayValue(req.Situation, "frozen_constraints")) > 0 {
		return namedDecision(req, "reject", "reject", "frozen_constraint_violation")
	}
	if numberValue(req.Situation, "components") > 1 && boolValue(req.Situation, "independent") {
		return namedDecision(req, "split", "decompose", "independent_components")
	}
	if recovery, ok := objectValue(req.Situation, "recovery"); ok {
		if stringValue(recovery, "previous_progress") == "material" &&
			stringValue(recovery, "failure_count") == "one" &&
			boolValue(recovery, "repair_bounded") {
			return namedDecision(req, "same-worker-repair", "retry", "material_progress", "bounded_repair")
		}
		if stringValue(recovery, "previous_progress") == "none" && stringValue(recovery, "failure_count") == "multiple" {
			return namedDecision(req, "specialist", "reroute", "zero_progress", "recurring_failure")
		}
	}
	if stringValue(req.Situation, "context_dependency") == "high" {
		return namedDecision(req, "strong-planner", "dispatch", "architectural_context")
	}

	best, ok := cheapestSufficient(req.Choices)
	if !ok {
		return routeDecision{}, errors.New("no eligible and sufficient choice")
	}
	if missingCapability(req.Situation) {
		return routeDecision{Schema: "telltail.decision.v1", CaseID: req.CaseID, Action: "dispatch", ChoiceID: best.ID, ReasonCodes: []string{"required_capability"}}, nil
	}
	return routeDecision{Schema: "telltail.decision.v1", CaseID: req.CaseID, Action: "dispatch", ChoiceID: best.ID, ReasonCodes: []string{"bounded_scope", "mechanical_acceptance"}}, nil
}

func namedDecision(req routeRequest, choiceID, action string, reasons ...string) (routeDecision, error) {
	for _, c := range req.Choices {
		if c.ID == choiceID {
			if !c.Eligible || !c.Sufficient {
				return routeDecision{}, fmt.Errorf("required route %q is not eligible and sufficient", choiceID)
			}
			return routeDecision{Schema: "telltail.decision.v1", CaseID: req.CaseID, Action: action, ChoiceID: choiceID, ReasonCodes: reasons}, nil
		}
	}
	return routeDecision{}, fmt.Errorf("required route %q is unavailable", choiceID)
}

func cheapestSufficient(choices []routeChoice) (routeChoice, bool) {
	eligible := make([]routeChoice, 0, len(choices))
	for _, c := range choices {
		if c.Eligible && c.Sufficient {
			eligible = append(eligible, c)
		}
	}
	if len(eligible) == 0 {
		return routeChoice{}, false
	}
	sort.SliceStable(eligible, func(i, j int) bool {
		if eligible[i].CostScore != eligible[j].CostScore {
			return eligible[i].CostScore < eligible[j].CostScore
		}
		if eligible[i].Priority != nil && eligible[j].Priority != nil && *eligible[i].Priority != *eligible[j].Priority {
			return *eligible[i].Priority < *eligible[j].Priority
		}
		return eligible[i].ID < eligible[j].ID
	})
	return eligible[0], true
}

func stringValue(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func boolValue(m map[string]any, key string) bool {
	v, _ := m[key].(bool)
	return v
}

func numberValue(m map[string]any, key string) float64 {
	v, _ := m[key].(float64)
	return v
}

func arrayValue(m map[string]any, key string) []any {
	v, _ := m[key].([]any)
	return v
}

func objectValue(m map[string]any, key string) (map[string]any, bool) {
	v, ok := m[key].(map[string]any)
	return v, ok
}

func missingCapability(situation map[string]any) bool {
	required := stringSet(situation, "required_capabilities")
	available := stringSet(situation, "cheap_worker_capabilities")
	if len(required) == 0 || len(available) == 0 {
		return false
	}
	for capability := range required {
		if !available[capability] {
			return true
		}
	}
	return false
}

func stringSet(m map[string]any, key string) map[string]bool {
	values, _ := m[key].([]any)
	set := map[string]bool{}
	for _, value := range values {
		if s, ok := value.(string); ok {
			set[s] = true
		}
	}
	return set
}
