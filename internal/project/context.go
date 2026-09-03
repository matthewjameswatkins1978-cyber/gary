package project

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// PlannerInput is the normal compact context for a Yardmaster decision.
func (c ProjectContext) PlannerInput() string {
	b, _ := json.MarshalIndent(struct {
		Project         Project          `json:"project"`
		State           ProjectState     `json:"state"`
		Manual          ProjectManual    `json:"manual"`
		AppliedFeedback []FeedbackRecord `json:"applied_feedback,omitempty"`
	}{c.Project, c.State, c.Manual, c.AppliedFeedback}, "", "  ")
	return "DURABLE GARY PROJECT CONTEXT\n=============================\n" + string(b)
}

// PlannerInputWithSource gives the Yardmaster both its working model and the
// immutable source, so source-level verification never depends on the manual
// being complete or lossless.
func (c ProjectContext) PlannerInputWithSource() string {
	return c.PlannerInput() + "\n\nORIGINAL MASTER PACKET (source of truth)\n=========================================\n" + string(c.Packet)
}

type ChildContext struct {
	ProjectID          string        `json:"project_id"`
	MasterPacketDigest string        `json:"master_packet_digest"`
	ManualVersion      int           `json:"project_manual_version"`
	ChildTaskID        string        `json:"child_task_id"`
	Task               string        `json:"task"`
	Objective          string        `json:"objective"`
	HardConstraints    []Requirement `json:"hard_constraints,omitempty"`
	Requirements       []Requirement `json:"relevant_requirements,omitempty"`
	AcceptanceCriteria []Requirement `json:"acceptance_criteria,omitempty"`
	Dependencies       []Requirement `json:"known_dependencies,omitempty"`
	ForbiddenChanges   []Requirement `json:"forbidden_changes,omitempty"`
}

// BuildChildContext keeps the always-safety-critical material and selects
// other requirements by deterministic keyword overlap with the child task.
func (c ProjectContext) BuildChildContext(task string) ChildContext {
	return c.BuildChildContextForID(task, "")
}

// BuildChildContextForID binds the existing GARY job identifier to the
// narrowed context without introducing a second worker identity scheme.
func (c ProjectContext) BuildChildContextForID(task, childTaskID string) ChildContext {
	ctx := ChildContext{
		ProjectID: c.Project.ProjectID, MasterPacketDigest: c.Project.MasterPacketDigest,
		ManualVersion: c.Project.ProjectManualVersion, ChildTaskID: childTaskID, Task: task,
		Objective: c.Manual.Objective, HardConstraints: c.Manual.HardConstraints,
		AcceptanceCriteria: c.Manual.AcceptanceCriteria, Dependencies: c.Manual.KnownDependencies,
		ForbiddenChanges: c.Manual.ForbiddenChanges,
	}
	for _, r := range append(append([]Requirement{}, c.Manual.FunctionalRequirements...), c.Manual.NonFunctionalRequirements...) {
		if relevant(r.Text, task) {
			ctx.Requirements = append(ctx.Requirements, r)
		}
	}
	return ctx
}

func (c ProjectContext) ChildTask(task string) string {
	return c.ChildTaskForID(task, "")
}

func (c ProjectContext) ChildTaskForID(task, childTaskID string) string {
	b, _ := json.MarshalIndent(c.BuildChildContextForID(task, childTaskID), "", "  ")
	return task + "\n\nDURABLE PROJECT EXECUTION CONTEXT\n=================================\n" + string(b)
}

var wordPattern = regexp.MustCompile(`[a-z0-9]+`)

func relevant(text, task string) bool {
	words := map[string]bool{}
	for _, word := range wordPattern.FindAllString(strings.ToLower(task), -1) {
		if len(word) >= 4 {
			words[word] = true
		}
	}
	for _, word := range wordPattern.FindAllString(strings.ToLower(text), -1) {
		if words[word] {
			return true
		}
	}
	return false
}

func (c ProjectContext) ValidateProvenance(projectID, digest string, version int) error {
	if c.Project.ProjectID != projectID || c.Project.MasterPacketDigest != digest || c.Project.ProjectManualVersion != version || c.Manual.ProjectID != projectID || c.Manual.SourcePacketDigest != digest || c.Manual.ManualVersion != version {
		return fmt.Errorf("%w: child context provenance mismatch", ErrProjectIDMismatch)
	}
	return nil
}
