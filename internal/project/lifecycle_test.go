package project

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestActiveProjectSupportsConcurrentActivitiesWithoutLifecycleTransition(t *testing.T) {
	store := NewStore(t.TempDir())
	ctx, err := store.CreateAndIngest(context.Background(), "activities", "Activities", "", "", []byte("packet"), testManual)
	if err != nil {
		t.Fatal(err)
	}
	startGeneration := ctx.Project.Generation
	state := ctx.State
	state.State = LifecycleActive
	state.Activities = []string{ActivityExecuting, ActivityVerifying, ActivityIntegrating}
	state.CurrentPhase = "mixed"
	state.LastEvent = "parallel project activities"
	if err := store.SaveState("activities", state); err != nil {
		t.Fatal(err)
	}
	ctx, err = store.LoadContext("activities")
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Project.State != LifecycleActive || ctx.Project.Generation != startGeneration {
		t.Fatalf("activity update changed lifecycle: project=%+v", ctx.Project)
	}
	for _, activity := range []string{ActivityExecuting, ActivityVerifying, ActivityIntegrating} {
		if !ctx.State.HasActivity(activity) {
			t.Fatalf("missing activity %s in %+v", activity, ctx.State.Activities)
		}
	}
	events, err := store.LoadLifecycleEvents("activities")
	if err != nil || len(events) != 1 {
		t.Fatalf("activity-only update created lifecycle event: %+v err=%v", events, err)
	}
}

func TestLifecycleTransitionsAppendEventsAndRejectStaleGeneration(t *testing.T) {
	store := NewStore(t.TempDir())
	ctx, err := store.CreateAndIngest(context.Background(), "generations", "Generations", "", "", []byte("packet"), testManual)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Project.Generation != 1 {
		t.Fatalf("initial generation=%d want 1", ctx.Project.Generation)
	}

	stale := ctx.State
	stale.Generation = 1

	next := ctx.State
	next.State = LifecycleParked
	next.Activities = nil
	next.Generation = 1
	next.Reason = "bounded run ended"
	if err := store.SaveState("generations", next); err != nil {
		t.Fatal(err)
	}
	ctx, err = store.LoadContext("generations")
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Project.State != LifecycleParked || ctx.Project.Generation != 2 {
		t.Fatalf("park transition = %+v", ctx.Project)
	}
	events, err := store.LoadLifecycleEvents("generations")
	if err != nil || len(events) != 2 || events[1].PreviousState != LifecycleActive || events[1].NewState != LifecycleParked || events[1].Generation != 2 {
		t.Fatalf("lifecycle events = %+v err=%v", events, err)
	}

	stale.State = LifecycleActive
	stale.Activities = []string{ActivityPlanning}
	stale.Reason = "stale resume"
	if err := store.SaveState("generations", stale); !errors.Is(err, ErrLifecycleStaleGeneration) {
		t.Fatalf("stale generation write err=%v", err)
	}
}

func TestLegacyLifecycleProjectionCompatibility(t *testing.T) {
	for _, tc := range []struct {
		legacy     string
		lifecycle  string
		activity   string
		archived   bool
	}{
		{"PLANNING", LifecycleActive, ActivityPlanning, false},
		{"EXECUTING", LifecycleActive, ActivityExecuting, false},
		{"VERIFYING", LifecycleActive, ActivityVerifying, false},
		{"PAUSED", LifecycleParked, "", false},
		{"FAILED", LifecycleParked, "", false},
		{"ARCHIVED", LifecycleAccepted, "", true},
	} {
		got, activities, archived := canonicalLifecycle(tc.legacy)
		if got != tc.lifecycle || archived != tc.archived {
			t.Fatalf("canonicalLifecycle(%s) = %s %v archived=%t", tc.legacy, got, activities, archived)
		}
		if tc.activity != "" && (len(activities) != 1 || activities[0] != tc.activity) {
			t.Fatalf("canonicalLifecycle(%s) activities=%v want %s", tc.legacy, activities, tc.activity)
		}
	}
}

func TestArchiveMetadataDoesNotChangeTerminalOutcome(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.CreateAndIngest(context.Background(), "archive-metadata", "Archive Metadata", "", "", []byte("packet"), testManual); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveState("archive-metadata", ProjectState{
		ProjectID: "archive-metadata", State: LifecycleActive,
		Activities: []string{ActivityVerifying}, CurrentPhase: "verification",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.AcceptProject("archive-metadata", "master acceptance passed"); err != nil {
		t.Fatal(err)
	}
	if err := store.ArchiveProject("archive-metadata"); err != nil {
		t.Fatal(err)
	}
	p, err := store.LoadProject("archive-metadata")
	if err != nil {
		t.Fatal(err)
	}
	if p.State != LifecycleAccepted || p.ArchivedAt == "" {
		t.Fatalf("archive changed accepted lifecycle: %+v", p)
	}
	archivePath := filepath.Join(store.Root(), "archive-metadata", "archive", "archive.json")
	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("archive metadata missing: %v", err)
	}
}

func TestIllegalLifecycleTransitionsFailClosed(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.CreateAndIngest(context.Background(), "illegal-transition", "Illegal Transition", "", "", []byte("packet"), testManual); err != nil {
		t.Fatal(err)
	}
	ctx, err := store.LoadContext("illegal-transition")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveState("illegal-transition", ProjectState{
		ProjectID: "illegal-transition", State: LifecycleParked, Generation: ctx.Project.Generation, Reason: "park",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveState("illegal-transition", ProjectState{
		ProjectID: "illegal-transition", State: LifecycleAccepted,
		Activities: []string{ActivityVerifying}, Reason: "illegal shortcut",
	}); !errors.Is(err, ErrProjectNotAcceptable) {
		t.Fatalf("PARKED -> ACCEPTED shortcut err=%v", err)
	}
}
