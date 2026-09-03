package project

import (
	"context"
	"sync"
	"testing"
	"time"
)

func watchdogProject(t *testing.T, id string) Store {
	t.Helper()
	store := NewStore(t.TempDir())
	if _, err := store.CreateAndIngest(context.Background(), id, id, "", "", []byte("packet"), testManual); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveState(id, ProjectState{ProjectID: id, State: LifecycleParked, CurrentPhase: "parked", RemainingCapabilities: []string{"finish the project"}, YardmasterState: YardmasterParked, ParkReason: ParkReasonWorkExhausted}); err != nil {
		t.Fatal(err)
	}
	return store
}

func TestAutoWakeClaimIsAtomicAndDoesNotTouchRunningProject(t *testing.T) {
	store := watchdogProject(t, "wake-race")
	var wg sync.WaitGroup
	decisions := make(chan WakeDecision, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			decision, err := store.ClaimAutoWake("wake-race", time.Unix(100, 0).UTC(), DefaultWatchdogPolicy())
			if err != nil {
				t.Errorf("ClaimAutoWake: %v", err)
				return
			}
			decisions <- decision
		}()
	}
	wg.Wait()
	close(decisions)
	requested := 0
	for decision := range decisions {
		if decision.Action == WakeRequestedAction {
			requested++
		}
	}
	if requested != 1 {
		t.Fatalf("atomic wake claims = %d, want 1", requested)
	}
	ctx, err := store.LoadContext("wake-race")
	if err != nil || ctx.State.YardmasterState != YardmasterWaking || ctx.State.Watchdog.Generation != 1 {
		t.Fatalf("claimed state = %+v err=%v", ctx.State, err)
	}

	if err := store.SaveState("wake-race", ProjectState{ProjectID: "wake-race", State: LifecycleActive, Activities: []string{ActivityExecuting}, CurrentPhase: "executing", YardmasterState: YardmasterRunning, RemainingCapabilities: []string{"still running"}}); err != nil {
		t.Fatal(err)
	}
	decision, err := store.ClaimAutoWake("wake-race", time.Unix(101, 0).UTC(), DefaultWatchdogPolicy())
	if err != nil || decision.Reason != NoActionRunning {
		t.Fatalf("running tick = %+v err=%v", decision, err)
	}
}

func TestAutoWakeSkipsLegitimateWaitingAndNoWork(t *testing.T) {
	store := watchdogProject(t, "wake-waiting")
	if err := store.SaveState("wake-waiting", ProjectState{ProjectID: "wake-waiting", State: LifecycleWaiting, YardmasterState: YardmasterParked, WaitingFor: "operator decision", RemainingCapabilities: []string{"work"}, Reason: "waiting for operator decision"}); err != nil {
		t.Fatal(err)
	}
	decision, err := store.ClaimAutoWake("wake-waiting", time.Unix(100, 0).UTC(), DefaultWatchdogPolicy())
	if err != nil || decision.Reason != NoActionWaiting {
		t.Fatalf("waiting tick = %+v err=%v", decision, err)
	}

	noWork := watchdogProject(t, "wake-no-work")
	ctx, err := noWork.LoadContext("wake-no-work")
	if err != nil {
		t.Fatal(err)
	}
	ctx.Manual.RemainingCapabilities = nil
	ctx.Manual.ManualVersion++
	if err := noWork.SaveManual("wake-no-work", ctx.Manual); err != nil {
		t.Fatal(err)
	}
	if err := noWork.SaveState("wake-no-work", ProjectState{ProjectID: "wake-no-work", State: LifecycleParked, YardmasterState: YardmasterParked}); err != nil {
		t.Fatal(err)
	}
	decision, err = noWork.ClaimAutoWake("wake-no-work", time.Unix(101, 0).UTC(), DefaultWatchdogPolicy())
	if err != nil || decision.Reason != NoActionNoWork {
		t.Fatalf("no-work tick = %+v err=%v", decision, err)
	}
}

func TestAutoWakeBackoffAndCircuitBreaker(t *testing.T) {
	store := watchdogProject(t, "wake-breaker")
	policy := WatchdogPolicy{MaxNoProgress: 2, Backoff: []time.Duration{time.Second}}
	now := time.Unix(100, 0).UTC()
	for attempt := 0; attempt < 2; attempt++ {
		decision, err := store.ClaimAutoWake("wake-breaker", now, policy)
		if err != nil || decision.Action != WakeRequestedAction {
			t.Fatalf("attempt %d claim = %+v err=%v", attempt+1, decision, err)
		}
		if err := store.StartAutoWake("wake-breaker", decision.WakeID, now); err != nil {
			t.Fatal(err)
		}
		if err := store.RecordAutoWakeOutcome("wake-breaker", decision.WakeID, now, WakeOutcome{}, policy); err != nil {
			t.Fatal(err)
		}
		if attempt == 0 {
			gated, err := store.ClaimAutoWake("wake-breaker", now, policy)
			if err != nil || gated.Reason != NoActionBackoff {
				t.Fatalf("backoff gate = %+v err=%v", gated, err)
			}
			now = now.Add(2 * time.Second)
		}
	}
	decision, err := store.ClaimAutoWake("wake-breaker", now, policy)
	if err != nil || decision.Reason != NoActionCircuit {
		t.Fatalf("circuit gate = %+v err=%v", decision, err)
	}
}

func TestAutoWakeOutcomeRecordsProgressAndKeepsBoardInProcess(t *testing.T) {
	store := watchdogProject(t, "wake-progress")
	now := time.Unix(100, 0).UTC()
	decision, err := store.ClaimAutoWake("wake-progress", now, DefaultWatchdogPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.StartAutoWake("wake-progress", decision.WakeID, now); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordAutoWakeOutcome("wake-progress", decision.WakeID, now, WakeOutcome{Progress: true}, DefaultWatchdogPolicy()); err != nil {
		t.Fatal(err)
	}
	ctx, err := store.LoadContext("wake-progress")
	if err != nil || ctx.State.Watchdog.LastOutcome != "PROGRESS" || ctx.State.YardmasterState != YardmasterParked || ctx.Project.State != LifecycleParked {
		t.Fatalf("progress outcome = %+v err=%v", ctx.State, err)
	}
	board, err := store.LoadBoard()
	if err != nil || board.Projects[0].Status != BoardInProcess {
		t.Fatalf("board after wake = %+v err=%v", board, err)
	}
}
