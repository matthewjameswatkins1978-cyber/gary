package project

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	LifecycleActive    = "ACTIVE"
	LifecycleParked    = "PARKED"
	LifecycleWaiting   = "WAITING"
	LifecycleBlocked   = "BLOCKED"
	LifecycleAccepted  = "ACCEPTED"
	LifecycleCancelled = "CANCELLED"

	ActivityPlanning    = "PLANNING"
	ActivityExecuting   = "EXECUTING"
	ActivityVerifying   = "VERIFYING"
	ActivityIntegrating = "INTEGRATING"

	LifecycleEventSchema = "gary.project-lifecycle-event.v1"
)

var ErrLifecycleStaleGeneration = errors.New("PROJECT_LIFECYCLE_STALE_GENERATION")

// LifecycleEvent is the append-only canonical history of project lifecycle
// transitions. state.json, project.json and the project board are projections.
type LifecycleEvent struct {
	Schema              string   `json:"schema"`
	EventID             string   `json:"event_id"`
	EventDigest         string   `json:"event_digest"`
	PreviousEventDigest string   `json:"previous_event_digest,omitempty"`
	ProjectID           string   `json:"project_id"`
	Generation          int      `json:"generation"`
	PreviousState       string   `json:"previous_state,omitempty"`
	NewState            string   `json:"new_state"`
	Actor               string   `json:"actor"`
	Reason              string   `json:"reason"`
	EvidenceRefs        []string `json:"evidence_refs,omitempty"`
	CreatedAt           string   `json:"created_at"`
}

func validLifecycleState(state string) bool {
	switch state {
	case LifecycleActive, LifecycleParked, LifecycleWaiting, LifecycleBlocked, LifecycleAccepted, LifecycleCancelled:
		return true
	default:
		return false
	}
}

func validActivity(activity string) bool {
	switch activity {
	case ActivityPlanning, ActivityExecuting, ActivityVerifying, ActivityIntegrating:
		return true
	default:
		return false
	}
}

// canonicalLifecycle accepts old on-disk project-state vocabulary so existing
// durable projects remain readable during migration. New writes are canonical.
func canonicalLifecycle(state string) (lifecycle string, inferredActivities []string, legacyArchived bool) {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case LifecycleActive:
		return LifecycleActive, nil, false
	case LifecycleParked, "PAUSED", "FAILED":
		return LifecycleParked, nil, false
	case LifecycleWaiting:
		return LifecycleWaiting, nil, false
	case LifecycleBlocked:
		return LifecycleBlocked, nil, false
	case LifecycleAccepted:
		return LifecycleAccepted, nil, false
	case LifecycleCancelled:
		return LifecycleCancelled, nil, false
	case "PLANNING":
		return LifecycleActive, []string{ActivityPlanning}, false
	case "EXECUTING":
		return LifecycleActive, []string{ActivityExecuting}, false
	case "VERIFYING":
		return LifecycleActive, []string{ActivityVerifying}, false
	case "INTEGRATING":
		return LifecycleActive, []string{ActivityIntegrating}, false
	case "ARCHIVED":
		// Historical ARCHIVED meant an accepted project moved to archive
		// storage. Preserve the successful outcome and expose archive separately.
		return LifecycleAccepted, nil, true
	default:
		return "", nil, false
	}
}

func inferActivityFromPhase(phase string) string {
	switch strings.ToLower(strings.TrimSpace(phase)) {
	case "planning", "intake":
		return ActivityPlanning
	case "executing", "running":
		return ActivityExecuting
	case "verification", "verifying", "final-verification", "final-inspection":
		return ActivityVerifying
	case "integrating", "integration":
		return ActivityIntegrating
	default:
		return ""
	}
}

func normalizeActivities(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToUpper(strings.TrimSpace(value))
		if value == "" || !validActivity(value) || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func normalizeProjectedState(state ProjectState) (ProjectState, bool, error) {
	lifecycle, inferred, archived := canonicalLifecycle(state.State)
	if lifecycle == "" {
		return ProjectState{}, false, fmt.Errorf("%w: invalid lifecycle state %q", ErrProjectMissing, state.State)
	}
	state.State = lifecycle
	if lifecycle == LifecycleActive {
		if len(state.Activities) == 0 {
			state.Activities = inferred
		}
		if len(state.Activities) == 0 {
			if activity := inferActivityFromPhase(state.CurrentPhase); activity != "" {
				state.Activities = []string{activity}
			}
		}
		state.Activities = normalizeActivities(state.Activities)
	} else {
		state.Activities = nil
	}
	return state, archived, nil
}

func (s ProjectState) HasActivity(activity string) bool {
	for _, value := range s.Activities {
		if strings.EqualFold(value, activity) {
			return true
		}
	}
	return false
}

func (s Store) lifecycleEventsPath(id string) string {
	return filepath.Join(s.projectDir(id), "events", "lifecycle.jsonl")
}

// LoadLifecycleEvents validates the append-only event chain.
func (s Store) LoadLifecycleEvents(id string) ([]LifecycleEvent, error) {
	path := s.lifecycleEventsPath(id)
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var events []LifecycleEvent
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024), 1<<20)
	previousDigest := ""
	previousGeneration := 0
	previousState := ""
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
		var event LifecycleEvent
		dec := json.NewDecoder(strings.NewReader(scanner.Text()))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&event); err != nil {
			return nil, fmt.Errorf("decode lifecycle event: %w", err)
		}
		if event.Schema != LifecycleEventSchema || event.ProjectID != id || event.EventID == "" || event.Actor == "" || event.Reason == "" || event.CreatedAt == "" {
			return nil, fmt.Errorf("invalid lifecycle event %q", event.EventID)
		}
		if event.EventDigest == "" || LifecycleEventDigest(event) != event.EventDigest {
			return nil, fmt.Errorf("invalid lifecycle event digest %q", event.EventID)
		}
		if event.PreviousEventDigest != previousDigest {
			return nil, fmt.Errorf("lifecycle event chain mismatch at %q", event.EventID)
		}
		if event.Generation != previousGeneration+1 {
			return nil, fmt.Errorf("lifecycle generation mismatch at %q", event.EventID)
		}
		if event.PreviousState != previousState {
			return nil, fmt.Errorf("lifecycle previous state mismatch at %q", event.EventID)
		}
		if !validLifecycleState(event.NewState) {
			return nil, fmt.Errorf("invalid lifecycle target %q", event.NewState)
		}
		events = append(events, event)
		previousDigest = event.EventDigest
		previousGeneration = event.Generation
		previousState = event.NewState
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func LifecycleEventDigest(event LifecycleEvent) string {
	event.EventDigest = ""
	b, _ := json.Marshal(event)
	return Digest(b)
}

func (s Store) appendLifecycleEventUnlocked(event LifecycleEvent) error {
	events, err := s.LoadLifecycleEvents(event.ProjectID)
	if err != nil {
		return err
	}
	previousDigest := ""
	previousState := ""
	previousGeneration := 0
	if len(events) > 0 {
		last := events[len(events)-1]
		previousDigest = last.EventDigest
		previousState = last.NewState
		previousGeneration = last.Generation
	}
	if event.Generation != previousGeneration+1 || event.PreviousState != previousState {
		return fmt.Errorf("%w: expected generation %d from %s", ErrLifecycleStaleGeneration, previousGeneration+1, previousState)
	}
	event.Schema = LifecycleEventSchema
	event.PreviousEventDigest = previousDigest
	if event.CreatedAt == "" {
		event.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if event.EventID == "" {
		event.EventID = fmt.Sprintf("%s-lifecycle-%06d", event.ProjectID, event.Generation)
	}
	event.EventDigest = LifecycleEventDigest(event)

	path := s.lifecycleEventsPath(event.ProjectID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

func (s Store) initialiseLifecycleEvent(id string, createdAt string) error {
	return s.appendLifecycleEventUnlocked(LifecycleEvent{
		ProjectID: id, Generation: 1, PreviousState: "", NewState: LifecycleActive,
		Actor: "control-plane", Reason: "project created", CreatedAt: createdAt,
	})
}
