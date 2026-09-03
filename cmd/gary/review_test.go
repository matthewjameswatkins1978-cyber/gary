package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/cost"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/jobs"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/project"
)

func TestProjectExplainIsReadOnlyAndEvidenceBound(t *testing.T) {
	root := t.TempDir()
	store := project.NewStore(root)
	if _, err := store.CreateAndIngest(context.Background(), "explain", "Explain", "", "", []byte("packet"), func(_ context.Context, ctx project.ProjectContext) (project.ProjectManual, error) {
		return project.ProjectManual{
			ProjectID: ctx.Project.ProjectID, Objective: "Explain decisions", DefinitionOfDone: "recorded evidence",
		}, nil
	}); err != nil {
		t.Fatal(err)
	}
	before := snapshotFiles(t, root)
	t.Setenv("GARY_PROJECTS_DIR", root)
	explanation, err := buildProjectExplanation("explain", "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if !explanation.ReadOnly || explanation.Packet.Digest == "" || explanation.Manual.Digest == "" || len(explanation.Basis) == 0 {
		t.Fatalf("explanation lacks read-only evidence boundary: %+v", explanation)
	}
	after := snapshotFiles(t, root)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("explain changed durable project files:\nbefore=%v\nafter=%v", before, after)
	}
}

func TestProjectExplainIncludesRecordedMasterEvidenceAndFilters(t *testing.T) {
	root := t.TempDir()
	work := t.TempDir()
	t.Chdir(work)
	store := project.NewStore(root)
	if _, err := store.CreateAndIngest(context.Background(), "explain", "Explain", "", "", []byte("packet"), func(_ context.Context, ctx project.ProjectContext) (project.ProjectManual, error) {
		return project.ProjectManual{ProjectID: ctx.Project.ProjectID, Objective: "Explain decisions", DefinitionOfDone: "recorded evidence"}, nil
	}); err != nil {
		t.Fatal(err)
	}
	status := masterStatus{
		ID: "master-1", ProjectID: "explain", State: "done", StagingBranch: "gary/master-1",
		Waves: []masterWaveRecord{{Wave: 2, PlannerMode: "control-direct", PlanSummary: "split the work", Results: []jobs.RunResult{{
			JobID: "child-1", FinalStatus: "DONE", Accepted: true, Branch: "gary/child-1", CommitSHA: "abc123", Attempts: 1,
			Cost: &cost.Summary{TotalCostUSD: 0.12},
		}}}},
	}
	data, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	statusPath := filepath.Join(work, "gary", "master", "status", "master-1.json")
	if err := os.MkdirAll(filepath.Dir(statusPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statusPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GARY_PROJECTS_DIR", root)
	explanation, err := buildProjectExplanation("explain", "master-1", "child-1", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(explanation.MasterRuns) != 1 || len(explanation.MasterRuns[0].Waves) != 1 || len(explanation.MasterRuns[0].Waves[0].Results) != 1 {
		t.Fatalf("filtered master evidence = %+v", explanation.MasterRuns)
	}
	result := explanation.MasterRuns[0].Waves[0].Results[0]
	if result.Branch != "gary/child-1" || result.CommitSHA != "abc123" || result.Cost.TotalCostUSD != 0.12 {
		t.Fatalf("worker evidence = %+v", result)
	}
	if !containsString(explanation.EvidenceSources, filepath.Join("gary", "master", "status", "master-1.json")) {
		t.Fatalf("master status was not cited: %v", explanation.EvidenceSources)
	}
}

func TestReviewCLIAcceptsProjectIDBeforeFlags(t *testing.T) {
	args, err := normalizeReviewArgs([]string{"demo", "--scope", "note", "--text", "why", "--json"}, map[string]bool{"--scope": true, "--text": true})
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 6 || args[0] != "--scope" || args[2] != "--text" || args[4] != "--json" || args[5] != "demo" {
		t.Fatalf("normalized review args = %v", args)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(value, want) {
			return true
		}
	}
	return false
}

func snapshotFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[rel] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}
