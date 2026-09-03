package project

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProjectPersistenceHelper(t *testing.T) {
	if os.Getenv("GARY_PROJECT_HELPER") != "1" {
		return
	}
	store := NewStore(os.Getenv("GARY_PROJECT_ROOT"))
	if _, err := store.CreateAndIngest(context.Background(), "restart", "Restart", "", "", []byte("packet"), testManual); err != nil {
		os.Exit(2)
	}
	if err := os.WriteFile(os.Getenv("GARY_PROJECT_READY"), []byte("ready"), 0o600); err != nil {
		os.Exit(3)
	}
	if os.Getenv("GARY_PROJECT_EXIT_ON_READY") == "1" {
		os.Exit(0)
	}
	select {}
}

func testManual(_ context.Context, ctx ProjectContext) (ProjectManual, error) {
	return ProjectManual{
		Schema: ManualSchema, ProjectID: ctx.Project.ProjectID,
		Objective: "Build the book burner", DefinitionOfDone: "Acceptance passes",
		HardConstraints:        []Requirement{{ID: "HC-001", Type: "hard_constraint", Text: "Normal capture requires no typing", Source: SourceRef{PacketDigest: ctx.Project.MasterPacketDigest, Locator: "MASTER_PACKET.md#capture"}}},
		FunctionalRequirements: []Requirement{{ID: "REQ-001", Type: "requirement", Text: "Implement ISBN scanning persistence", Source: SourceRef{PacketDigest: ctx.Project.MasterPacketDigest, Locator: "MASTER_PACKET.md#isbn"}}},
		AcceptanceCriteria:     []Requirement{{ID: "ACC-001", Type: "acceptance", Text: "The focused test suite passes", Source: SourceRef{PacketDigest: ctx.Project.MasterPacketDigest, Locator: "MASTER_PACKET.md#acceptance"}}},
		CurrentPhase:           "planning", RemainingCapabilities: []string{"ISBN scanning"},
	}, nil
}

func TestCreateAndIngestPersistsBeforeGeneratorAndRecovers(t *testing.T) {
	store := NewStore(t.TempDir())
	packet := []byte("# Book Burner\n\nNormal capture requires no typing.\n")
	called := false
	ctx, err := store.CreateAndIngest(context.Background(), "book-burner", "Book Burner", "", "", packet, func(ctx context.Context, input ProjectContext) (ProjectManual, error) {
		called = true
		got, err := store.LoadMasterPacket(input.Project.ProjectID)
		if err != nil || string(got) != string(packet) {
			t.Fatalf("packet was not persisted before generation: %v", err)
		}
		return testManual(ctx, input)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called || ctx.Project.MasterPacketDigest != Digest(packet) || ctx.Project.ProjectManualVersion != 1 || ctx.Manual.ManualVersion != 1 {
		t.Fatalf("unexpected ingested context: called=%t context=%+v", called, ctx)
	}
	recovered, err := store.LoadContext("book-burner")
	if err != nil {
		t.Fatal(err)
	}
	if string(recovered.Packet) != string(packet) || recovered.Manual.Objective != "Build the book burner" {
		t.Fatalf("recovered context lost data: %+v", recovered)
	}
	board, err := store.LoadBoard()
	if err != nil || len(board.Projects) != 1 || board.Projects[0].Status != BoardInProcess || board.Projects[0].ProjectManualVersion != 1 {
		t.Fatalf("project board after start = %+v err=%v", board, err)
	}
	resumed, err := store.ResumeProject("book-burner")
	if err != nil || resumed.Project.State != LifecycleActive || resumed.State.State != LifecycleActive || !resumed.State.HasActivity(ActivityPlanning) {
		t.Fatalf("resume context = %+v err=%v", resumed, err)
	}
}

func TestProjectBoardCompletesOnlyAfterProjectAcceptance(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.CreateAndIngest(context.Background(), "board-project", "Board project", "", "", []byte("packet"), testManual); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkProjectCompleted(Project{ProjectID: "board-project", State: "PAUSED"}); err == nil {
		t.Fatal("paused project was marked completed")
	}
	board, err := store.LoadBoard()
	if err != nil || board.Projects[0].Status != BoardInProcess {
		t.Fatalf("board changed after rejected completion: %+v err=%v", board, err)
	}
	if err := store.SaveState("board-project", ProjectState{ProjectID: "board-project", State: LifecycleActive, Activities: []string{ActivityVerifying}, CurrentPhase: "verification"}); err != nil {
		t.Fatal(err)
	}
	if err := store.AcceptProject("board-project", "accepted"); err != nil {
		t.Fatal(err)
	}
	board, err = store.LoadBoard()
	if err != nil || len(board.Projects) != 1 || board.Projects[0].Status != BoardCompleted || board.Projects[0].CompletedAt == "" {
		t.Fatalf("board after acceptance = %+v err=%v", board, err)
	}
	if err := store.MarkProjectInProcess(Project{ProjectID: "board-project", ProjectName: "stale", State: "ACCEPTED", MasterPacketDigest: "stale"}); err != nil {
		t.Fatal(err)
	}
	board, err = store.LoadBoard()
	if err != nil || board.Projects[0].Status != BoardCompleted {
		t.Fatalf("completed board entry was resurrected: %+v err=%v", board, err)
	}
}

func TestProjectBoardDoesNotResurrectCancelledEntry(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.CreateAndIngest(context.Background(), "cancelled-board", "Cancelled board", "", "", []byte("packet"), testManual); err != nil {
		t.Fatal(err)
	}
	if err := store.CancelProject("cancelled-board", "explicit cancellation"); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkProjectCompleted(Project{ProjectID: "cancelled-board", State: LifecycleAccepted}); err != nil {
		t.Fatal(err)
	}
	board, err := store.LoadBoard()
	if err != nil || board.Projects[0].Status != BoardCancelled {
		t.Fatalf("cancelled board entry was resurrected: %+v err=%v", board, err)
	}
}

func TestLoadContextRejectsInvalidLifecycleProjection(t *testing.T) {
	for _, tc := range []struct {
		name  string
		state string
	}{
		{name: "waiting", state: LifecycleWaiting},
		{name: "blocked", state: LifecycleBlocked},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := NewStore(t.TempDir())
			if _, err := store.CreateAndIngest(context.Background(), tc.name, tc.name, "", "", []byte("packet"), testManual); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(store.Root(), tc.name, "state.json")
			var raw map[string]any
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Fatal(err)
			}
			raw["state"] = tc.state
			delete(raw, "waiting_for")
			delete(raw, "blocker")
			data, err = json.Marshal(raw)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := store.LoadContext(tc.name); !errors.Is(err, ErrProjectNotAcceptable) {
				t.Fatalf("invalid %s projection loaded: %v", tc.state, err)
			}
		})
	}
}

func TestLoadContextRejectsStateProjectGenerationMismatch(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.CreateAndIngest(context.Background(), "generation-mismatch", "Generation mismatch", "", "", []byte("packet"), testManual); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(store.Root(), "generation-mismatch", "state.json")
	var raw map[string]any
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	raw["generation"] = float64(99)
	data, err = json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadContext("generation-mismatch"); !errors.Is(err, ErrLifecycleStaleGeneration) {
		t.Fatalf("generation mismatch was accepted: %v", err)
	}
}

func TestIncompleteIngestionRetainsPacketButCannotPlan(t *testing.T) {
	store := NewStore(t.TempDir())
	packet := []byte("# Incomplete project\n")
	if _, err := store.CreateAndIngest(context.Background(), "incomplete", "Incomplete", "", "", packet, func(context.Context, ProjectContext) (ProjectManual, error) {
		return ProjectManual{}, errors.New("manual generator unavailable")
	}); err == nil {
		t.Fatal("manual generation unexpectedly succeeded")
	}
	if got, err := store.LoadMasterPacket("incomplete"); err != nil || string(got) != string(packet) {
		t.Fatalf("packet retention = %q err=%v", got, err)
	}
	if _, err := store.LoadContext("incomplete"); !strings.Contains(err.Error(), ErrManualMissing.Error()) {
		t.Fatalf("incomplete project was usable: %v", err)
	}
}

func TestPendingIngestionCanBeRetriedAndNormalizesBoardPolicy(t *testing.T) {
	store := NewStore(t.TempDir())
	packet := []byte("# Retryable intake\n")
	if _, err := store.CreateAndIngest(context.Background(), "retryable", "Retryable", "", "", packet, func(context.Context, ProjectContext) (ProjectManual, error) {
		return ProjectManual{}, errors.New("temporary Yardmaster output failure")
	}); err == nil {
		t.Fatal("initial intake unexpectedly succeeded")
	}

	ctx, err := store.IngestPending(context.Background(), "retryable", func(_ context.Context, input ProjectContext) (ProjectManual, error) {
		manual, err := testManual(context.Background(), input)
		if err != nil {
			return ProjectManual{}, err
		}
		manual.BoardPolicy = BoardPolicy{InProcessStatus: BoardInProcess}
		return manual, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Project.ProjectManualVersion != 1 || ctx.State.State != LifecycleActive || !ctx.State.HasActivity(ActivityPlanning) || ctx.Manual.BoardPolicy != DefaultBoardPolicy() {
		t.Fatalf("retried intake = %+v", ctx)
	}
	if got, err := store.LoadMasterPacket("retryable"); err != nil || string(got) != string(packet) {
		t.Fatalf("retry changed immutable packet: %q err=%v", got, err)
	}
	board, err := store.LoadBoard()
	if err != nil || len(board.Projects) != 1 || board.Projects[0].Status != BoardInProcess || board.Projects[0].ProjectManualVersion != 1 {
		t.Fatalf("board after retried intake = %+v err=%v", board, err)
	}
}

func TestContradictionsRemainExplicitAndTraceable(t *testing.T) {
	store := NewStore(t.TempDir())
	packet := []byte("# Conflicting project\nOption A\nOption B\n")
	ctx, err := store.CreateAndIngest(context.Background(), "conflict", "Conflict", "", "", packet, func(_ context.Context, input ProjectContext) (ProjectManual, error) {
		m, err := testManual(context.Background(), input)
		if err != nil {
			return ProjectManual{}, err
		}
		m.Conflicts = []Conflict{{ID: "CONFLICT-001", Subject: "routing mode", Options: []string{"A", "B"}, Status: "CONTRADICTORY", Source: SourceRef{PacketDigest: input.Project.MasterPacketDigest, Locator: "MASTER_PACKET.md#routing"}}}
		return m, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(ctx.Manual.Conflicts) != 1 || ctx.Manual.Conflicts[0].Status != "CONTRADICTORY" || ctx.Manual.Conflicts[0].Source.Locator != "MASTER_PACKET.md#routing" {
		t.Fatalf("contradiction was not preserved: %+v", ctx.Manual.Conflicts)
	}
}

func TestProcessDeathDoesNotEraseProjectContext(t *testing.T) {
	root := t.TempDir()
	ready := filepath.Join(root, "ready")
	cmd := exec.Command(os.Args[0], "-test.run=TestProjectPersistenceHelper")
	cmd.Env = append(os.Environ(), "GARY_PROJECT_HELPER=1", "GARY_PROJECT_ROOT="+root, "GARY_PROJECT_READY="+ready, "GARY_PROJECT_EXIT_ON_READY=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("helper did not persist project before process kill")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("helper process exit = %v", err)
	}
	ctx, err := NewStore(root).LoadContext("restart")
	if err != nil || ctx.Project.ProjectID != "restart" || ctx.Manual.Objective != "Build the book burner" {
		t.Fatalf("context after process death = %+v err=%v", ctx, err)
	}
}

func TestMissingAndTamperedPacketFailClosed(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.CreateAndIngest(context.Background(), "tamper", "Tamper", "", "", []byte("packet"), testManual); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(store.Root(), "tamper", "source", "MASTER_PACKET.md")
	if err := os.WriteFile(path, []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadContext("tamper"); !strings.Contains(err.Error(), ErrPacketDigestMismatch.Error()) {
		t.Fatalf("tampered packet error = %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadMasterPacket("tamper"); !strings.Contains(err.Error(), ErrPacketMissing.Error()) {
		t.Fatalf("missing packet error = %v", err)
	}
}

func TestManualTamperAndIDMismatchFailClosed(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.CreateAndIngest(context.Background(), "manual", "Manual", "", "", []byte("packet"), testManual); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(store.Root(), "manual", "manuals", "project-manual-v001.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(b), "Build the book burner", "Changed", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadCurrentManual("manual"); !strings.Contains(err.Error(), ErrManualInvalid.Error()) {
		t.Fatalf("tampered manual error = %v", err)
	}
}

func TestAmendmentVersionsAndArchiveLifecycle(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.CreateAndIngest(context.Background(), "amend", "Amend", "", "", []byte("packet"), testManual); err != nil {
		t.Fatal(err)
	}
	old, err := os.ReadFile(filepath.Join(store.Root(), "amend", "manuals", "project-manual-v001.json"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := store.AmendAndIngest(context.Background(), "amend", []byte("Clarify ISBN acceptance."), func(_ context.Context, input ProjectContext) (ProjectManual, error) {
		m := input.Manual
		m.Objective = "Build the amended book burner"
		return m, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Project.ProjectManualVersion != 2 || ctx.Manual.ManualVersion != 2 || ctx.Manual.Objective != "Build the amended book burner" {
		t.Fatalf("amendment did not create version 2: %+v", ctx)
	}
	currentOld, err := os.ReadFile(filepath.Join(store.Root(), "amend", "manuals", "project-manual-v001.json"))
	if err != nil || string(currentOld) != string(old) {
		t.Fatal("historical manual was rewritten")
	}
	if err := store.AcceptProject("amend", "too early"); !strings.Contains(err.Error(), ErrProjectNotAcceptable.Error()) {
		t.Fatalf("project accepted before verification: %v", err)
	}
	if err := store.SaveState("amend", ProjectState{ProjectID: "amend", State: LifecycleActive, Activities: []string{ActivityVerifying}, CurrentPhase: "verification"}); err != nil {
		t.Fatal(err)
	}
	if err := store.AcceptProject("amend", "final acceptance passed"); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveState("amend", ProjectState{ProjectID: "amend", State: LifecycleActive, Activities: []string{ActivityPlanning}, CurrentPhase: "planning"}); !strings.Contains(err.Error(), ErrProjectNotAcceptable.Error()) {
		t.Fatalf("accepted project was resurrected to planning: %v", err)
	}
	if err := store.ArchiveProject("amend"); err != nil {
		t.Fatal(err)
	}
	final, err := store.LoadProject("amend")
	if err != nil || final.State != LifecycleAccepted || final.ArchivedAt == "" {
		t.Fatalf("archive metadata = %+v err=%v", final, err)
	}
	if final.MasterPacketDigest == "" || final.ProjectManualVersion != 2 || final.ManualDigest == "" {
		t.Fatalf("final project lost acceptance provenance: %+v", final)
	}
	if _, err := store.AddAmendment("amend", []byte("post-acceptance change")); !strings.Contains(err.Error(), ErrProjectNotAcceptable.Error()) {
		t.Fatalf("archived project accepted amendment: %v", err)
	}
	if err := store.SaveState("amend", ProjectState{ProjectID: "amend", State: LifecycleParked, CurrentPhase: "parked"}); !strings.Contains(err.Error(), ErrProjectNotAcceptable.Error()) {
		t.Fatalf("accepted archived project was resurrected to parked: %v", err)
	}
}

func TestChildContextIsNarrowAndProvenanceBound(t *testing.T) {
	store := NewStore(t.TempDir())
	ctx, err := store.CreateAndIngest(context.Background(), "narrow", "Narrow", "", "", []byte("packet"), testManual)
	if err != nil {
		t.Fatal(err)
	}
	child := ctx.BuildChildContext("Implement ISBN scanning persistence")
	if len(child.Requirements) != 1 || !strings.Contains(child.Requirements[0].Text, "ISBN") {
		t.Fatalf("unexpected relevant requirements: %+v", child.Requirements)
	}
	if err := ctx.ValidateProvenance("narrow", ctx.Project.MasterPacketDigest, 1); err != nil {
		t.Fatal(err)
	}
	if err := ctx.ValidateProvenance("narrow", "sha256:wrong", 1); err == nil {
		t.Fatal("wrong packet provenance accepted")
	}
	childTask := ctx.ChildTaskForID("Implement ISBN scanning persistence", "job-001")
	if strings.Contains(childTask, "Normal capture requires no typing") == false || !strings.Contains(childTask, `"child_task_id": "job-001"`) {
		t.Fatal("hard constraint was not preserved in child context")
	}
}

func TestProjectCancellationIsTerminalAndNotCompletion(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.CreateAndIngest(context.Background(), "cancelled-project", "Cancelled project", "", "", []byte("packet"), testManual); err != nil {
		t.Fatal(err)
	}
	if err := store.CancelProject("cancelled-project", "operator deliberately closed unfinished work"); err != nil {
		t.Fatal(err)
	}
	p, err := store.LoadProject("cancelled-project")
	if err != nil || p.State != LifecycleCancelled || p.CancelledAt == "" {
		t.Fatalf("cancelled project = %+v err=%v", p, err)
	}
	board, err := store.LoadBoard()
	if err != nil || len(board.Projects) != 1 || board.Projects[0].Status != BoardCancelled || board.Projects[0].CompletedAt != "" {
		t.Fatalf("cancelled board projection = %+v err=%v", board, err)
	}
	if err := store.SaveState("cancelled-project", ProjectState{ProjectID: "cancelled-project", State: LifecycleActive, Activities: []string{ActivityPlanning}}); !strings.Contains(err.Error(), ErrProjectNotAcceptable.Error()) {
		t.Fatalf("cancelled project was resurrected: %v", err)
	}
	if err := store.ArchiveProject("cancelled-project"); err != nil {
		t.Fatal(err)
	}
	p, err = store.LoadProject("cancelled-project")
	if err != nil || p.State != LifecycleCancelled || p.ArchivedAt == "" {
		t.Fatalf("archiving changed cancelled outcome: %+v err=%v", p, err)
	}
}
