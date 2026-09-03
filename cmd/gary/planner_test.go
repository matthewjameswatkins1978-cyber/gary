package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestYardmasterRequestBodyUsesUserRole(t *testing.T) {
	payload, err := json.Marshal(yardmasterRequestBody("plan this"))
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	var body struct {
		Contents []struct {
			Role  string `json:"role"`
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}
	if len(body.Contents) != 1 {
		t.Fatalf("contents length = %d, want 1", len(body.Contents))
	}
	if body.Contents[0].Role != "user" {
		t.Fatalf("contents[0].role = %q, want user", body.Contents[0].Role)
	}
	if len(body.Contents[0].Parts) != 1 || body.Contents[0].Parts[0].Text != "plan this" {
		t.Fatalf("unexpected contents[0].parts: %#v", body.Contents[0].Parts)
	}
}

func TestYardmasterVertexEndpointUsesGlobalPublisherModel(t *testing.T) {
	got := yardmasterVertexEndpoint("gary-agent-yard", "gemini-3.8-flash")
	want := "https://aiplatform.googleapis.com/v1/projects/gary-agent-yard/locations/global/publishers/google/models/gemini-3.8-flash:generateContent"
	if got != want {
		t.Fatalf("endpoint = %q, want %q", got, want)
	}
}

func TestYardmasterVertexEndpointDoesNotUseDeveloperAPI(t *testing.T) {
	got := yardmasterVertexEndpoint("p", "m")
	if strings.Contains(got, "generativelanguage.googleapis.com") {
		t.Fatalf("Yardmaster must use Vertex control-plane auth, got %q", got)
	}
	if !strings.Contains(got, "/locations/global/") {
		t.Fatalf("Yardmaster should use global Vertex endpoint, got %q", got)
	}
}

func TestPlannerDisposablePathsAreExcludedFromContext(t *testing.T) {
	for _, test := range []struct {
		path string
		want bool
	}{
		{path: ".venv/lib/python/site.py", want: true},
		{path: "nested/__pycache__/module.pyc", want: true},
		{path: "BookBurner.egg-info/PKG-INFO", want: true},
		{path: "BookBurner/Features/Capture/CaptureStateMachine.swift", want: false},
	} {
		if got := plannerDisposablePath(test.path); got != test.want {
			t.Fatalf("plannerDisposablePath(%q) = %t, want %t", test.path, got, test.want)
		}
	}
}

func TestWorkerSupervisorPromptContainsLiveControlDoctrine(t *testing.T) {
	data, err := os.ReadFile("planner.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, marker := range []string{
		"missing ordinary tool or package",
		"STOP_AND_REPLAN when observable evidence shows",
		"READY_FOR_INSPECTION means leave implementation mode",
		"not itself acceptance",
		"authority the worker does not have",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("worker supervisor prompt missing %q", marker)
		}
	}
}


func TestSmallFixDoctrineHasBoundedYardmasterRepairAndReverification(t *testing.T) {
	plannerData, err := os.ReadFile("planner.go")
	if err != nil {
		t.Fatal(err)
	}
	plannerSource := string(plannerData)
	for _, marker := range []string{
		"runYardmasterSmallRepair",
		"one bounded direct repair",
		"git-apply compatible",
		"substantial implementation",
	} {
		if !strings.Contains(plannerSource, marker) {
			t.Fatalf("Yardmaster SMALL_FIX planner path missing %q", marker)
		}
	}

	runData, err := os.ReadFile("run.go")
	if err != nil {
		t.Fatal(err)
	}
	runSource := string(runData)
	for _, marker := range []string{
		"runYardmasterSmallRepair(cfg, job, workDir, quality)",
		"verifyExistingCandidate(workDir, job)",
		"runChildQualityInspection(cfg, job, workDir)",
		"YardmasterRepaired:",
		"yardmaster-repair.patch",
	} {
		if !strings.Contains(runSource, marker) {
			t.Fatalf("Yardmaster SMALL_FIX acceptance path missing %q", marker)
		}
	}
}
