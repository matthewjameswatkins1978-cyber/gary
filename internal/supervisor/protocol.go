package supervisor

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type Action string

const (
	ActionContinue          Action = "CONTINUE"
	ActionGuide             Action = "GUIDE"
	ActionStopAndReplan     Action = "STOP_AND_REPLAN"
	ActionReadyForInspection Action = "READY_FOR_INSPECTION"
)

type Telemetry struct {
	SchemaVersion       int               `json:"schema_version"`
	JobID               string            `json:"job_id"`
	ReviewID            string            `json:"review_id"`
	Phase               string            `json:"phase"`
	Turn                int               `json:"turn"`
	Timestamp            string            `json:"timestamp"`
	ElapsedActiveSeconds int               `json:"elapsed_active_seconds"`
	SessionID           string            `json:"session_id,omitempty"`
	GeminiExit          *int              `json:"gemini_exit,omitempty"`
	AcceptanceExit      *int              `json:"acceptance_exit,omitempty"`
	ChangedFiles        int               `json:"changed_files"`
	DiffBytes           int               `json:"diff_bytes"`
	DiffSHA256          string            `json:"diff_sha256,omitempty"`
	DiffStat            string            `json:"diff_stat,omitempty"`
	GitStatus           string            `json:"git_status,omitempty"`
	AcceptanceTail      string            `json:"acceptance_tail,omitempty"`
	ToolCalls           map[string]int    `json:"tool_calls,omitempty"`
	Errors              []string          `json:"errors,omitempty"`
}

type Decision struct {
	SchemaVersion int    `json:"schema_version"`
	Generation    int    `json:"generation"`
	ReviewID      string `json:"review_id"`
	Action        Action `json:"action"`
	Guidance      string `json:"guidance,omitempty"`
	Reason        string `json:"reason,omitempty"`
	IssuedAt      string `json:"issued_at"`
}

func (d *Decision) Normalize(generation int, reviewID string, now time.Time) error {
	if d.SchemaVersion == 0 {
		d.SchemaVersion = 1
	}
	d.Generation = generation
	d.ReviewID = reviewID
	d.IssuedAt = now.UTC().Format(time.RFC3339)
	d.Guidance = strings.TrimSpace(d.Guidance)
	d.Reason = strings.TrimSpace(d.Reason)
	switch d.Action {
	case ActionContinue, ActionGuide, ActionStopAndReplan, ActionReadyForInspection:
	default:
		return fmt.Errorf("unsupported supervision action %q", d.Action)
	}
	if d.Action == ActionGuide && d.Guidance == "" {
		return errors.New("GUIDE requires non-empty guidance")
	}
	if d.Action != ActionGuide && len(d.Guidance) > 2000 {
		return errors.New("supervision guidance is too large")
	}
	if len(d.Guidance) > 4000 || len(d.Reason) > 2000 {
		return errors.New("supervision decision text exceeds bounds")
	}
	return nil
}

func (t Telemetry) Validate() error {
	if t.SchemaVersion != 1 {
		return fmt.Errorf("unsupported telemetry schema %d", t.SchemaVersion)
	}
	if strings.TrimSpace(t.JobID) == "" || strings.TrimSpace(t.ReviewID) == "" {
		return errors.New("telemetry requires job_id and review_id")
	}
	switch t.Phase {
	case "running", "turn_complete", "candidate_ready", "stopping":
	default:
		return fmt.Errorf("unsupported telemetry phase %q", t.Phase)
	}
	if t.Turn < 1 {
		return errors.New("telemetry turn must be positive")
	}
	return nil
}
