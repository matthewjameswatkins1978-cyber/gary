package project

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const (
	YardmasterRunning = "RUNNING"
	YardmasterParked  = "PARKED"
	YardmasterWaking  = "WAKING"

	CompletionActive           = "ACTIVE"
	CompletionApprovalPending  = "APPROVAL_PENDING"
	CompletionApprovedFinished = "APPROVED_FINISHED"

	ParkReasonWorkExhausted = "WORK_EXHAUSTED"
	ParkReasonWaiting       = "WAITING"
	ParkReasonBlocked       = "BLOCKED"
	ParkReasonNoProgress    = "NO_MEANINGFUL_PROGRESS"

	WakeRequested = "REQUESTED"
	WakeStarted   = "STARTED"
	WakeCompleted = "COMPLETED"
	WakeFailed    = "FAILED"
)

// WatchdogState is mutable evidence about one-shot wake attempts. The wake ID
// and generation make duplicate ticks observable and let the claim remain
// atomic even when multiple schedulers notice the same parked project.
type WatchdogState struct {
	Generation              int    `json:"generation,omitempty"`
	WakeID                  string `json:"wake_id,omitempty"`
	Status                  string `json:"status,omitempty"`
	RequestedAt             string `json:"requested_at,omitempty"`
	StartedAt               string `json:"started_at,omitempty"`
	CompletedAt             string `json:"completed_at,omitempty"`
	FailedAt                string `json:"failed_at,omitempty"`
	FailureCount            int    `json:"failure_count,omitempty"`
	NoProgressCount         int    `json:"no_progress_count,omitempty"`
	CircuitBreakerOpen      bool   `json:"circuit_breaker_open,omitempty"`
	NextEligibleWakeAt      string `json:"next_eligible_wake_at,omitempty"`
	LastProgressFingerprint string `json:"last_progress_fingerprint,omitempty"`
	LastOutcome             string `json:"last_outcome,omitempty"`
	LastError               string `json:"last_error,omitempty"`
}

// WatchdogPolicy is deliberately conservative. A tick is cheap, but a wake
// may invoke the paid Yardmaster path, so repeated no-progress wakes back off
// and then stop until a human or a later explicit reset intervenes.
type WatchdogPolicy struct {
	MaxNoProgress int
	Backoff       []time.Duration
}

func DefaultWatchdogPolicy() WatchdogPolicy {
	return WatchdogPolicy{MaxNoProgress: 3, Backoff: []time.Duration{time.Minute, 5 * time.Minute}}
}

type WakeDecision struct {
	Action     string `json:"action"`
	Reason     string `json:"reason"`
	ProjectID  string `json:"project_id"`
	WakeID     string `json:"wake_id,omitempty"`
	Generation int    `json:"generation,omitempty"`
}

const (
	WakeRequestedAction = "WAKE_REQUESTED"
	NoActionApproved    = "NO_ACTION_APPROVED_FINISHED"
	NoActionRunning     = "NO_ACTION_RUNNING"
	NoActionWaking      = "NO_ACTION_ALREADY_WAKING"
	NoActionNotParked   = "NO_ACTION_NOT_PARKED"
	NoActionNoWork      = "NO_ACTION_NO_EXECUTABLE_WORK"
	NoActionWaiting     = "NO_ACTION_WAITING"
	NoActionBlocked     = "NO_ACTION_BLOCKED"
	NoActionBackoff     = "NO_ACTION_BACKOFF"
	NoActionCircuit     = "NO_ACTION_CIRCUIT_BREAKER"
)

func (p WatchdogPolicy) normalized() WatchdogPolicy {
	if p.MaxNoProgress < 1 {
		p.MaxNoProgress = 3
	}
	if len(p.Backoff) == 0 {
		p.Backoff = DefaultWatchdogPolicy().Backoff
	}
	return p
}

func (s ProjectState) normalized(projectState string) ProjectState {
	if s.Completion == "" {
		if projectState == "ACCEPTED" || projectState == "ARCHIVED" {
			s.Completion = CompletionApprovedFinished
		} else if projectState == "VERIFYING" {
			s.Completion = CompletionApprovalPending
		} else {
			s.Completion = CompletionActive
		}
	}
	if s.YardmasterState == "" {
		switch projectState {
		case "PAUSED", "FAILED":
			s.YardmasterState = YardmasterParked
		default:
			s.YardmasterState = YardmasterRunning
		}
	}
	return s
}

// HasExecutableWork is intentionally stricter than “not finished”: only
// explicit remaining or outstanding work is eligible for automatic wake.
func (c ProjectContext) HasExecutableWork() bool {
	return len(c.State.RemainingCapabilities) > 0 || len(c.State.OutstandingWork) > 0 || len(c.Manual.RemainingCapabilities) > 0
}

func (s Store) ClaimAutoWake(id string, now time.Time, policy WatchdogPolicy) (WakeDecision, error) {
	policy = policy.normalized()
	var decision WakeDecision
	err := s.withProjectStateLock(id, func() error {
		ctx, err := s.LoadContext(id)
		if err != nil {
			return err
		}
		state := ctx.State.normalized(ctx.Project.State)
		decision = WakeDecision{ProjectID: id}
		switch {
		case state.Completion == CompletionApprovedFinished || ctx.Project.State == "ACCEPTED" || ctx.Project.State == "ARCHIVED":
			decision.Action, decision.Reason = "NO_ACTION", NoActionApproved
		case state.YardmasterState == YardmasterRunning:
			decision.Action, decision.Reason = "NO_ACTION", NoActionRunning
		case state.YardmasterState == YardmasterWaking:
			decision.Action, decision.Reason = "NO_ACTION", NoActionWaking
		case state.YardmasterState != YardmasterParked:
			decision.Action, decision.Reason = "NO_ACTION", NoActionNotParked
		case !ctx.HasExecutableWork():
			decision.Action, decision.Reason = "NO_ACTION", NoActionNoWork
		case strings.TrimSpace(state.WaitingFor) != "":
			decision.Action, decision.Reason = "NO_ACTION", NoActionWaiting
		case strings.TrimSpace(state.ExternalDependency) != "":
			decision.Action, decision.Reason = "NO_ACTION", NoActionWaiting
		case strings.TrimSpace(state.BlockedUntil) != "" && after(now, state.BlockedUntil):
			decision.Action, decision.Reason = "NO_ACTION", NoActionBlocked
		case state.Watchdog.CircuitBreakerOpen:
			decision.Action, decision.Reason = "NO_ACTION", NoActionCircuit
		case strings.TrimSpace(state.Watchdog.NextEligibleWakeAt) != "" && after(now, state.Watchdog.NextEligibleWakeAt):
			decision.Action, decision.Reason = "NO_ACTION", NoActionBackoff
		default:
			state.YardmasterState = YardmasterWaking
			state.Watchdog.Generation++
			state.Watchdog.WakeID = fmt.Sprintf("%s-wake-%06d", id, state.Watchdog.Generation)
			state.Watchdog.Status = WakeRequested
			state.Watchdog.RequestedAt = now.UTC().Format(time.RFC3339Nano)
			state.Watchdog.StartedAt = ""
			state.Watchdog.CompletedAt = ""
			state.Watchdog.FailedAt = ""
			state.Watchdog.LastError = ""
			state.LastEvent = "auto-wake requested"
			if err := s.saveStateUnlocked(id, state); err != nil {
				return err
			}
			decision = WakeDecision{Action: WakeRequestedAction, Reason: "PARKED_WITH_EXECUTABLE_WORK", ProjectID: id, WakeID: state.Watchdog.WakeID, Generation: state.Watchdog.Generation}
		}
		return nil
	})
	return decision, err
}

func (s Store) StartAutoWake(id, wakeID string, now time.Time) error {
	return s.withProjectStateLock(id, func() error {
		ctx, err := s.LoadContext(id)
		if err != nil {
			return err
		}
		state := ctx.State.normalized(ctx.Project.State)
		if state.YardmasterState != YardmasterWaking || state.Watchdog.WakeID != wakeID || state.Watchdog.Status != WakeRequested {
			return fmt.Errorf("wake %s is not the current requested wake", wakeID)
		}
		state.State = "PLANNING"
		state.CurrentPhase = "planning"
		state.YardmasterState = YardmasterRunning
		state.Watchdog.Status = WakeStarted
		state.Watchdog.StartedAt = now.UTC().Format(time.RFC3339Nano)
		state.LastEvent = "auto-wake started Yardmaster resume"
		return s.saveStateUnlocked(id, state)
	})
}

type WakeOutcome struct {
	Progress bool
	Failed   bool
	Error    string
}

func (s Store) RecordAutoWakeOutcome(id, wakeID string, now time.Time, outcome WakeOutcome, policy WatchdogPolicy) error {
	policy = policy.normalized()
	return s.withProjectStateLock(id, func() error {
		ctx, err := s.LoadContext(id)
		if err != nil {
			return err
		}
		state := ctx.State.normalized(ctx.Project.State)
		if state.Watchdog.WakeID != wakeID || state.Watchdog.Status != WakeStarted {
			return fmt.Errorf("wake %s is not the current started wake", wakeID)
		}
		stamp := now.UTC().Format(time.RFC3339Nano)
		state.Watchdog.CompletedAt = stamp
		state.Watchdog.FailedAt = ""
		state.Watchdog.LastError = strings.TrimSpace(outcome.Error)
		if outcome.Failed {
			state.Watchdog.Status = WakeFailed
			state.Watchdog.FailedAt = stamp
			state.Watchdog.FailureCount++
			state.Watchdog.NoProgressCount++
			state.Watchdog.LastOutcome = "FAILED"
			state.ParkReason = ParkReasonNoProgress
			state.LastEvent = "auto-wake failed"
		} else {
			state.Watchdog.Status = WakeCompleted
			state.Watchdog.LastOutcome = "PROGRESS"
			if outcome.Progress {
				state.Watchdog.FailureCount = 0
				state.Watchdog.NoProgressCount = 0
				state.Watchdog.CircuitBreakerOpen = false
				state.Watchdog.NextEligibleWakeAt = ""
				state.Watchdog.LastProgressFingerprint = ctx.ProgressFingerprint()
				state.ParkReason = ParkReasonWorkExhausted
			} else {
				state.Watchdog.NoProgressCount++
				state.Watchdog.FailureCount++
				state.Watchdog.LastOutcome = "NO_PROGRESS"
				state.ParkReason = ParkReasonNoProgress
			}
			state.LastEvent = "auto-wake completed"
		}
		if state.Watchdog.NoProgressCount >= policy.MaxNoProgress {
			state.Watchdog.CircuitBreakerOpen = true
			state.Watchdog.NextEligibleWakeAt = ""
		} else if !outcome.Progress {
			index := state.Watchdog.NoProgressCount - 1
			if index >= len(policy.Backoff) {
				index = len(policy.Backoff) - 1
			}
			state.Watchdog.NextEligibleWakeAt = now.Add(policy.Backoff[index]).UTC().Format(time.RFC3339Nano)
		}
		if ctx.Project.State != "ACCEPTED" && ctx.Project.State != "ARCHIVED" {
			state.YardmasterState = YardmasterParked
		}
		return s.saveStateUnlocked(id, state)
	})
}

func (c ProjectContext) ProgressFingerprint() string {
	b := []byte(c.Manual.ManualDigest + "\x00" + strings.Join(c.State.CompletedCapabilities, "\x00") + "\x00" + strings.Join(c.State.RemainingCapabilities, "\x00") + "\x00" + strings.Join(c.State.OutstandingWork, "\x00"))
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:])
}

func after(now time.Time, timestamp string) bool {
	t, err := time.Parse(time.RFC3339Nano, timestamp)
	return err == nil && now.Before(t)
}

func (s Store) withProjectStateLock(id string, fn func() error) error {
	if !IsValidID(id) {
		return fmt.Errorf("invalid project_id %q", id)
	}
	return withProjectFileLock(s.projectDir(id)+"/.state.lock", 5*time.Second, fn)
}
