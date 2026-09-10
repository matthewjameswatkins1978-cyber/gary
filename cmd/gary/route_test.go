package main

import "testing"

func TestDecideRouteSafetyAndPolicy(t *testing.T) {
	tests := []struct {
		name      string
		situation map[string]any
		choices   []routeChoice
		action    string
		choice    string
	}{
		{"cheap", map[string]any{}, []routeChoice{{ID: "cheap", CostScore: 1, Eligible: true, Sufficient: true}, {ID: "strong", CostScore: 5, Eligible: true, Sufficient: true}}, "dispatch", "cheap"},
		{"context", map[string]any{"context_dependency": "high"}, []routeChoice{{ID: "cheap", Eligible: true, Sufficient: false}, {ID: "strong-planner", Eligible: true, Sufficient: true}}, "dispatch", "strong-planner"},
		{"repair", map[string]any{"recovery": map[string]any{"previous_progress": "material", "failure_count": "one", "repair_bounded": true}}, []routeChoice{{ID: "same-worker-repair", Eligible: true, Sufficient: true}}, "retry", "same-worker-repair"},
		{"reject", map[string]any{"candidate_violates": true, "frozen_constraints": []any{"no-backend-change"}}, []routeChoice{{ID: "reject", Eligible: true, Sufficient: true}}, "reject", "reject"},
		{"interrupt", map[string]any{"reversible": false, "external_impact": "high", "worker_authority": false}, []routeChoice{{ID: "interrupt", Eligible: true, Sufficient: true}}, "interrupt", "interrupt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decideRoute(routeRequest{Protocol: telltailAdapterProtocol, CaseID: tt.name, ProbeKind: "route", Situation: tt.situation, Choices: tt.choices})
			if err != nil {
				t.Fatal(err)
			}
			if got.Action != tt.action || got.ChoiceID != tt.choice {
				t.Fatalf("got action=%q choice=%q, want action=%q choice=%q", got.Action, got.ChoiceID, tt.action, tt.choice)
			}
		})
	}
}

func TestDecideRouteFailsClosed(t *testing.T) {
	_, err := decideRoute(routeRequest{Protocol: "wrong", CaseID: "c", ProbeKind: "route", Choices: []routeChoice{{ID: "c"}}})
	if err == nil {
		t.Fatal("unsupported protocol accepted")
	}
	_, err = decideRoute(routeRequest{Protocol: telltailAdapterProtocol, CaseID: "c", ProbeKind: "route", Choices: []routeChoice{{ID: "cheap", Eligible: false, Sufficient: false}}})
	if err == nil {
		t.Fatal("no usable route accepted")
	}
}

func TestDecideRouteRejectsMalformedRequests(t *testing.T) {
	base := routeRequest{Protocol: telltailAdapterProtocol, CaseID: "c", ProbeKind: "route", Choices: []routeChoice{{ID: "c", Eligible: true, Sufficient: true}}}
	for name, req := range map[string]routeRequest{
		"missing-case":       {Protocol: telltailAdapterProtocol, ProbeKind: "route", Choices: base.Choices},
		"wrong-kind":         {Protocol: telltailAdapterProtocol, CaseID: "c", ProbeKind: "live", Choices: base.Choices},
		"no-choices":         {Protocol: telltailAdapterProtocol, CaseID: "c", ProbeKind: "route"},
		"duplicate-id":       {Protocol: telltailAdapterProtocol, CaseID: "c", ProbeKind: "route", Choices: []routeChoice{{ID: "c"}, {ID: "c"}}},
		"missing-specialist": {Protocol: telltailAdapterProtocol, CaseID: "c", ProbeKind: "route", Situation: map[string]any{"context_dependency": "high"}, Choices: base.Choices},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decideRoute(req); err == nil {
				t.Fatal("malformed request accepted")
			}
		})
	}
}

func TestCertifiedRouteBeatsCheaperCandidate(t *testing.T) {
	got, err := decideRoute(routeRequest{Protocol: telltailAdapterProtocol, CaseID: "certification", ProbeKind: "route", Choices: []routeChoice{
		{ID: "cheap-candidate", CostScore: 1, Eligible: true, Sufficient: true, Certification: "CANDIDATE"},
		{ID: "proven", CostScore: 9, Eligible: true, Sufficient: true, Certification: "CERTIFIED"},
	}})
	if err != nil || got.ChoiceID != "proven" {
		t.Fatalf("route=%+v err=%v; certified route should win", got, err)
	}
}
