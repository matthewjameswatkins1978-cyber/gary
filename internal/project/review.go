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
	FeedbackSchema = "gary.project-feedback-event.v1"

	FeedbackScopeNote              = "note"
	FeedbackScopeProjectPreference = "project_preference"
	FeedbackScopeProjectCorrection = "project_correction"
	FeedbackScopeProposeDoctrine   = "propose_doctrine"

	FeedbackEventSubmitted = "submitted"
	FeedbackEventApplied   = "applied"
	FeedbackRecorded       = "RECORDED"
	FeedbackPending        = "PENDING_SAFE_BOUNDARY"
	FeedbackApplied        = "APPLIED"
)

var (
	ErrReviewUnsafeBoundary = errors.New("REVIEW_UNSAFE_BOUNDARY")
	ErrFeedbackInvalid      = errors.New("FEEDBACK_INVALID")
)

// FeedbackEvent is one immutable line in the project review event stream.
// Application is represented by a second event; no submitted event is edited.
type FeedbackEvent struct {
	Schema              string `json:"schema"`
	EventID             string `json:"event_id"`
	EventType           string `json:"event_type"`
	PreviousEventDigest string `json:"previous_event_digest,omitempty"`
	EventDigest         string `json:"event_digest"`
	FeedbackID          string `json:"feedback_id"`
	ProjectID           string `json:"project_id"`
	Actor               string `json:"actor"`
	CreatedAt           string `json:"created_at"`
	Scope               string `json:"scope"`
	DecisionReference   string `json:"decision_reference,omitempty"`
	ChildTaskID         string `json:"child_task_id,omitempty"`
	Text                string `json:"text"`
	Status              string `json:"status"`
	AppliedAt           string `json:"applied_at,omitempty"`
	AppliedBoundary     string `json:"applied_boundary,omitempty"`
}

// FeedbackRecord is the current folded view of one feedback item. It is
// derived from immutable events and is safe to expose in inspect/status output.
type FeedbackRecord struct {
	FeedbackID        string `json:"feedback_id"`
	ProjectID         string `json:"project_id"`
	Actor             string `json:"actor"`
	CreatedAt         string `json:"created_at"`
	Scope             string `json:"scope"`
	DecisionReference string `json:"decision_reference,omitempty"`
	ChildTaskID       string `json:"child_task_id,omitempty"`
	Text              string `json:"text"`
	Status            string `json:"status"`
	AppliedAt         string `json:"applied_at,omitempty"`
	AppliedBoundary   string `json:"applied_boundary,omitempty"`
}

func feedbackPath(s Store, id string) string {
	return filepath.Join(s.projectDir(id), "feedback", "events.jsonl")
}

// AppendFeedback records explicit human feedback without changing the packet,
// manual, state, worker packets, or doctrine.
func (s Store) AppendFeedback(id, actor, scope, decisionReference, childTaskID, text string) (FeedbackRecord, error) {
	p, err := s.LoadProject(id)
	if err != nil {
		return FeedbackRecord{}, err
	}
	if strings.TrimSpace(actor) == "" || strings.TrimSpace(text) == "" {
		return FeedbackRecord{}, fmt.Errorf("%w: actor and text are required", ErrFeedbackInvalid)
	}
	if err := validateFeedbackScope(scope); err != nil {
		return FeedbackRecord{}, err
	}
	if terminalReviewState(p.State) && feedbackInitialStatus(scope) == FeedbackPending {
		return FeedbackRecord{}, fmt.Errorf("%w: behavioral feedback cannot be applied to terminal project state %s", ErrProjectNotAcceptable, p.State)
	}
	if len([]rune(actor)) > 200 || len([]rune(text)) > 16000 || len([]rune(decisionReference)) > 200 || len([]rune(childTaskID)) > 200 {
		return FeedbackRecord{}, fmt.Errorf("%w: feedback field is too long", ErrFeedbackInvalid)
	}

	now := time.Now().UTC()
	createdAt := now.Format(time.RFC3339Nano)
	base := FeedbackRecord{
		ProjectID: id, Actor: strings.TrimSpace(actor), CreatedAt: createdAt,
		Scope: scope, DecisionReference: strings.TrimSpace(decisionReference),
		ChildTaskID: strings.TrimSpace(childTaskID), Text: strings.TrimSpace(text),
		Status: feedbackInitialStatus(scope),
	}
	events, err := s.LoadFeedbackEvents(id)
	if err != nil {
		return FeedbackRecord{}, err
	}
	feedbackID := fmt.Sprintf("feedback-%d-%s", now.UnixNano(), strings.TrimPrefix(Digest([]byte(fmt.Sprintf("%s\x00%s\x00%s", scope, base.Text, base.Actor))), "sha256:")[:12])
	for _, event := range events {
		if event.FeedbackID == feedbackID {
			return FeedbackRecord{}, fmt.Errorf("%w: generated feedback id collision", ErrFeedbackInvalid)
		}
	}
	base.FeedbackID = feedbackID
	event := FeedbackEvent{
		Schema: FeedbackSchema, EventID: feedbackID + "-submitted", EventType: FeedbackEventSubmitted,
		PreviousEventDigest: previousFeedbackDigest(events),
		FeedbackID:          feedbackID, ProjectID: id, Actor: base.Actor, CreatedAt: createdAt,
		Scope: base.Scope, DecisionReference: base.DecisionReference, ChildTaskID: base.ChildTaskID,
		Text: base.Text, Status: base.Status,
	}
	if err := s.appendFeedbackEvent(event); err != nil {
		return FeedbackRecord{}, err
	}
	return base, nil
}

// LoadFeedbackEvents returns the raw append-only event stream and validates
// each transition. Missing streams are valid for projects created earlier.
func (s Store) LoadFeedbackEvents(id string) ([]FeedbackEvent, error) {
	if _, err := s.LoadProject(id); err != nil {
		return nil, err
	}
	f, err := os.Open(feedbackPath(s, id))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var events []FeedbackEvent
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024), 1<<20)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
		var event FeedbackEvent
		dec := json.NewDecoder(strings.NewReader(scanner.Text()))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&event); err != nil {
			return nil, fmt.Errorf("%w: decode feedback event: %v", ErrFeedbackInvalid, err)
		}
		if err := validateFeedbackEvent(event); err != nil {
			return nil, err
		}
		if event.ProjectID != id {
			return nil, fmt.Errorf("%w: event %q belongs to project %q", ErrFeedbackInvalid, event.EventID, event.ProjectID)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if err := validateFeedbackTransitions(events); err != nil {
		return nil, err
	}
	return events, nil
}

// LoadFeedback folds the event stream into the current review records.
func (s Store) LoadFeedback(id string) ([]FeedbackRecord, error) {
	events, err := s.LoadFeedbackEvents(id)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]FeedbackRecord)
	order := make([]string, 0)
	for _, event := range events {
		record := FeedbackRecord{
			FeedbackID: event.FeedbackID, ProjectID: event.ProjectID, Actor: event.Actor,
			CreatedAt: event.CreatedAt, Scope: event.Scope, DecisionReference: event.DecisionReference,
			ChildTaskID: event.ChildTaskID, Text: event.Text, Status: event.Status,
			AppliedAt: event.AppliedAt, AppliedBoundary: event.AppliedBoundary,
		}
		if _, exists := byID[event.FeedbackID]; !exists {
			order = append(order, event.FeedbackID)
		}
		byID[event.FeedbackID] = record
	}
	result := make([]FeedbackRecord, 0, len(order))
	for _, id := range order {
		result = append(result, byID[id])
	}
	return result, nil
}

// AppliedFeedback returns only behaviour-bearing feedback that has crossed a
// safe planning boundary. Notes and doctrine proposals remain observational.
func (s Store) AppliedFeedback(id string) ([]FeedbackRecord, error) {
	records, err := s.LoadFeedback(id)
	if err != nil {
		return nil, err
	}
	var applied []FeedbackRecord
	for _, record := range records {
		if record.Status == FeedbackApplied {
			applied = append(applied, record)
		}
	}
	return applied, nil
}

// ApplyPendingFeedback applies preference/correction records only while the
// project is explicitly at PLANNING. It writes application events and never
// rewrites a worker packet or the immutable packet/manual.
func (s Store) ApplyPendingFeedback(id, boundary string) ([]FeedbackRecord, error) {
	p, err := s.LoadProject(id)
	if err != nil {
		return nil, err
	}
	if p.State != "PLANNING" {
		return nil, fmt.Errorf("%w: project state is %s", ErrReviewUnsafeBoundary, p.State)
	}
	if strings.TrimSpace(boundary) == "" {
		return nil, fmt.Errorf("%w: boundary is required", ErrReviewUnsafeBoundary)
	}
	events, err := s.LoadFeedbackEvents(id)
	if err != nil {
		return nil, err
	}
	records, err := s.LoadFeedback(id)
	if err != nil {
		return nil, err
	}
	previousDigest := previousFeedbackDigest(events)
	for _, record := range records {
		if record.Status != FeedbackPending {
			continue
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		event := FeedbackEvent{
			Schema: FeedbackSchema, EventID: record.FeedbackID + "-applied-" + strings.TrimPrefix(Digest([]byte(now)), "sha256:")[:12],
			EventType: FeedbackEventApplied, FeedbackID: record.FeedbackID, ProjectID: id,
			PreviousEventDigest: previousDigest,
			Actor:               record.Actor, CreatedAt: record.CreatedAt, Scope: record.Scope,
			DecisionReference: record.DecisionReference, ChildTaskID: record.ChildTaskID,
			Text: record.Text, Status: FeedbackApplied, AppliedAt: now, AppliedBoundary: boundary,
		}
		event.EventDigest = FeedbackEventDigest(event)
		if err := s.appendFeedbackEvent(event); err != nil {
			return nil, err
		}
		previousDigest = event.EventDigest
	}
	return s.LoadFeedback(id)
}

func (s Store) appendFeedbackEvent(event FeedbackEvent) error {
	if event.EventDigest == "" {
		event.EventDigest = FeedbackEventDigest(event)
	}
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	path := feedbackPath(s, event.ProjectID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

func validateFeedbackScope(scope string) error {
	switch scope {
	case FeedbackScopeNote, FeedbackScopeProjectPreference, FeedbackScopeProjectCorrection, FeedbackScopeProposeDoctrine:
		return nil
	default:
		return fmt.Errorf("%w: invalid scope %q", ErrFeedbackInvalid, scope)
	}
}

func feedbackInitialStatus(scope string) string {
	if scope == FeedbackScopeProjectPreference || scope == FeedbackScopeProjectCorrection {
		return FeedbackPending
	}
	return FeedbackRecorded
}

func terminalReviewState(state string) bool {
	return state == "ACCEPTED" || state == "ARCHIVED"
}

func validateFeedbackEvent(event FeedbackEvent) error {
	if event.Schema != FeedbackSchema || event.EventID == "" || event.EventDigest == "" || event.EventDigest != FeedbackEventDigest(event) || event.FeedbackID == "" || event.ProjectID == "" || event.Actor == "" || event.CreatedAt == "" || event.Text == "" {
		return fmt.Errorf("%w: incomplete feedback event %q", ErrFeedbackInvalid, event.EventID)
	}
	if err := validateFeedbackScope(event.Scope); err != nil {
		return err
	}
	switch event.EventType {
	case FeedbackEventSubmitted:
		if event.Status != feedbackInitialStatus(event.Scope) || event.AppliedAt != "" || event.AppliedBoundary != "" {
			return fmt.Errorf("%w: invalid submitted event %q", ErrFeedbackInvalid, event.EventID)
		}
	case FeedbackEventApplied:
		if event.Status != FeedbackApplied || event.AppliedAt == "" || event.AppliedBoundary == "" || (event.Scope != FeedbackScopeProjectPreference && event.Scope != FeedbackScopeProjectCorrection) {
			return fmt.Errorf("%w: invalid applied event %q", ErrFeedbackInvalid, event.EventID)
		}
	default:
		return fmt.Errorf("%w: invalid event type %q", ErrFeedbackInvalid, event.EventType)
	}
	return nil
}

func validateFeedbackTransitions(events []FeedbackEvent) error {
	seen := map[string]FeedbackEvent{}
	eventIDs := map[string]bool{}
	previousDigest := ""
	for _, event := range events {
		if event.PreviousEventDigest != previousDigest {
			return fmt.Errorf("%w: feedback event chain mismatch at %q", ErrFeedbackInvalid, event.EventID)
		}
		previousDigest = event.EventDigest
		if eventIDs[event.EventID] {
			return fmt.Errorf("%w: repeated event id %q", ErrFeedbackInvalid, event.EventID)
		}
		eventIDs[event.EventID] = true
		prior, exists := seen[event.FeedbackID]
		switch event.EventType {
		case FeedbackEventSubmitted:
			if exists {
				return fmt.Errorf("%w: repeated feedback submission %q", ErrFeedbackInvalid, event.FeedbackID)
			}
			seen[event.FeedbackID] = event
		case FeedbackEventApplied:
			if !exists || prior.EventType != FeedbackEventSubmitted || prior.Status != FeedbackPending {
				return fmt.Errorf("%w: feedback %q applied without pending submission", ErrFeedbackInvalid, event.FeedbackID)
			}
			if event.ProjectID != prior.ProjectID || event.Actor != prior.Actor || event.CreatedAt != prior.CreatedAt || event.Scope != prior.Scope || event.DecisionReference != prior.DecisionReference || event.ChildTaskID != prior.ChildTaskID || event.Text != prior.Text {
				return fmt.Errorf("%w: applied event changed submitted feedback %q", ErrFeedbackInvalid, event.FeedbackID)
			}
			seen[event.FeedbackID] = event
		default:
			return fmt.Errorf("%w: invalid transition for %q", ErrFeedbackInvalid, event.FeedbackID)
		}
	}
	return nil
}

// FeedbackEventDigest makes edits, deletion, and reordering of the local
// review stream detectable without introducing a database or service.
func FeedbackEventDigest(event FeedbackEvent) string {
	event.EventDigest = ""
	b, _ := json.Marshal(event)
	return Digest(b)
}

func previousFeedbackDigest(events []FeedbackEvent) string {
	if len(events) == 0 {
		return ""
	}
	return events[len(events)-1].EventDigest
}
