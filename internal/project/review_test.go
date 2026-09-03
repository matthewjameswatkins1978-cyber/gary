package project

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newReviewProject(t *testing.T) (Store, ProjectContext) {
	t.Helper()
	store := NewStore(t.TempDir())
	ctx, err := store.CreateAndIngest(context.Background(), "review", "Review", "", "", []byte("# Review project\n"), testManual)
	if err != nil {
		t.Fatal(err)
	}
	return store, ctx
}

func TestReviewFeedbackScopesAndPlannerVisibility(t *testing.T) {
	store, ctx := newReviewProject(t)
	for _, input := range []struct {
		scope string
		text  string
	}{
		{FeedbackScopeNote, "The worker packet felt too large."},
		{FeedbackScopeProjectPreference, "Prefer smaller worker packets."},
		{FeedbackScopeProjectCorrection, "Split coupled jobs at the next planning point."},
		{FeedbackScopeProposeDoctrine, "Consider a global small-packet rule."},
	} {
		if _, err := store.AppendFeedback("review", "matthew", input.scope, "wave-1", "job-1", input.text); err != nil {
			t.Fatal(err)
		}
	}
	records, err := store.LoadFeedback("review")
	if err != nil || len(records) != 4 {
		t.Fatalf("feedback records = %+v err=%v", records, err)
	}
	if records[0].Status != FeedbackRecorded || records[1].Status != FeedbackPending || records[2].Status != FeedbackPending || records[3].Status != FeedbackRecorded {
		t.Fatalf("initial feedback statuses = %+v", records)
	}
	if got := ctx.PlannerInput(); strings.Contains(got, "smaller worker packets") || strings.Contains(got, "global small-packet") {
		t.Fatalf("unapplied feedback leaked into planner input: %s", got)
	}

	if err := store.SaveState("review", ProjectState{ProjectID: "review", State: LifecycleActive, Activities: []string{ActivityExecuting}, CurrentPhase: "executing"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ApplyPendingFeedback("review", "master-planning-boundary"); err == nil || !strings.Contains(err.Error(), ErrReviewUnsafeBoundary.Error()) {
		t.Fatalf("unsafe feedback application error = %v", err)
	}
	if err := store.SaveState("review", ProjectState{ProjectID: "review", State: LifecycleActive, Activities: []string{ActivityPlanning}, CurrentPhase: "planning"}); err != nil {
		t.Fatal(err)
	}
	records, err = store.ApplyPendingFeedback("review", "master-planning-boundary")
	if err != nil {
		t.Fatal(err)
	}
	if records[1].Status != FeedbackApplied || records[2].Status != FeedbackApplied || records[1].AppliedBoundary != "master-planning-boundary" {
		t.Fatalf("applied feedback = %+v", records)
	}
	applied, err := store.AppliedFeedback("review")
	if err != nil || len(applied) != 2 {
		t.Fatalf("applied feedback = %+v err=%v", applied, err)
	}
	loaded, err := store.LoadContext("review")
	if err != nil {
		t.Fatal(err)
	}
	planner := loaded.PlannerInput()
	if !strings.Contains(planner, "smaller worker packets") || !strings.Contains(planner, "Split coupled jobs") {
		t.Fatalf("applied feedback missing from planner input: %s", planner)
	}
	if strings.Contains(planner, "worker packet felt too large") || strings.Contains(planner, "global small-packet") {
		t.Fatalf("observation/doctrine proposal changed planner input: %s", planner)
	}
	if len(loaded.AppliedFeedback) != 2 {
		t.Fatalf("context applied feedback = %+v", loaded.AppliedFeedback)
	}
}

func TestReviewApplicationIsAppendOnlyAndDoesNotMutateWorkerContext(t *testing.T) {
	store, ctx := newReviewProject(t)
	childBefore := ctx.ChildTaskForID("Implement ISBN scanning persistence", "job-1")
	packetPath := filepath.Join(store.Root(), "review", "source", "MASTER_PACKET.md")
	manualPath := filepath.Join(store.Root(), "review", "manuals", "project-manual-v001.json")
	packetBefore, _ := os.ReadFile(packetPath)
	manualBefore, _ := os.ReadFile(manualPath)
	if _, err := store.AppendFeedback("review", "matthew", FeedbackScopeProjectPreference, "planner-decision", "job-1", "Prefer a smaller packet."); err != nil {
		t.Fatal(err)
	}
	eventsPath := filepath.Join(store.Root(), "review", "feedback", "events.jsonl")
	firstEvent, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveState("review", ProjectState{ProjectID: "review", State: LifecycleActive, Activities: []string{ActivityPlanning}, CurrentPhase: "planning"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ApplyPendingFeedback("review", "safe-boundary"); err != nil {
		t.Fatal(err)
	}
	allEvents, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(allEvents), string(firstEvent)) || strings.Count(string(allEvents), "\n") != 2 {
		t.Fatalf("feedback stream was not append-only: %q", allEvents)
	}
	packetAfter, _ := os.ReadFile(packetPath)
	manualAfter, _ := os.ReadFile(manualPath)
	if string(packetBefore) != string(packetAfter) || string(manualBefore) != string(manualAfter) {
		t.Fatal("feedback application mutated packet or manual")
	}
	if got := ctx.ChildTaskForID("Implement ISBN scanning persistence", "job-1"); got != childBefore {
		t.Fatal("in-flight child context changed after feedback application")
	}
}

func TestReviewEventsFailClosedOnInvalidDoctrineApplication(t *testing.T) {
	store, _ := newReviewProject(t)
	if _, err := store.AppendFeedback("review", "matthew", FeedbackScopeNote, "", "", "keep the observation traceable"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(store.Root(), "review", "feedback", "events.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(data), "keep the observation traceable", "promote", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadFeedback("review"); err == nil || !strings.Contains(err.Error(), ErrFeedbackInvalid.Error()) {
		t.Fatalf("tampered review stream accepted: %v", err)
	}
}
