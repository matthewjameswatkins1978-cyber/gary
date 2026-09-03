package project

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// ManualGenerator is the Yardmaster boundary. Keeping it injectable makes
// persistence and fail-closed behaviour cheap to test without a model call.
type ManualGenerator func(context.Context, ProjectContext) (ProjectManual, error)

// CreateAndIngest persists the packet before invoking the generator. If the
// generator fails, the project remains recoverable with manual version zero.
func (s Store) CreateAndIngest(ctx context.Context, id, name, repo, baseBranch string, packet []byte, generate ManualGenerator) (ProjectContext, error) {
	p, err := s.CreateProject(id, name, repo, baseBranch, packet)
	if err != nil {
		return ProjectContext{}, err
	}
	if generate == nil {
		return ProjectContext{}, fmt.Errorf("project %s persisted but no manual generator was supplied", id)
	}
	input := ProjectContext{Project: p, State: ProjectState{Schema: StateSchema, ProjectID: id, State: LifecycleActive, Generation: p.Generation, Activities: []string{ActivityPlanning}, CurrentPhase: "intake"}, Packet: append([]byte(nil), packet...)}
	manual, err := generate(ctx, input)
	if err != nil {
		return input, err
	}
	manual.ProjectID = id
	manual.SourcePacketDigest = p.MasterPacketDigest
	manual.ManualVersion = 1
	if err := s.SaveManual(id, manual); err != nil {
		return input, err
	}
	state := input.State
	state.State = LifecycleActive
	state.Activities = []string{ActivityPlanning}
	state.CurrentPhase = "planning"
	state.LastEvent = "project intake completed"
	state.Reason = "durable project manual created; ready for planning"
	state.UpdatedBy = "Yardmaster"
	if err := s.SaveState(id, state); err != nil {
		return ProjectContext{}, err
	}
	return s.LoadContext(id)
}

// IngestPending completes a project whose immutable packet was persisted but
// whose first manual generation failed. It never rewrites the packet and it
// refuses to replace an existing manual; this makes retrying an interrupted
// intake safe and keeps the normal Yardmaster path in charge.
func (s Store) IngestPending(ctx context.Context, id string, generate ManualGenerator) (ProjectContext, error) {
	p, err := s.LoadProject(id)
	if err != nil {
		return ProjectContext{}, err
	}
	if p.ProjectManualVersion != 0 {
		return s.LoadContext(id)
	}
	if generate == nil {
		return ProjectContext{}, fmt.Errorf("project %s has no manual and no manual generator was supplied", id)
	}
	packet, err := s.LoadMasterPacket(id)
	if err != nil {
		return ProjectContext{}, err
	}
	var state ProjectState
	if err := readJSON(filepath.Join(s.projectDir(id), "state.json"), &state); err != nil {
		return ProjectContext{}, fmt.Errorf("%w: state: %v", ErrProjectMissing, err)
	}
	if (state.Schema != StateSchema && state.Schema != LegacyStateSchema) || state.ProjectID != id {
		return ProjectContext{}, fmt.Errorf("%w: invalid project state", ErrProjectMissing)
	}
	state.Schema = StateSchema
	amendments, err := s.LoadAmendments(id)
	if err != nil {
		return ProjectContext{}, err
	}
	input := ProjectContext{Project: p, State: state, Amendments: amendments, Packet: append([]byte(nil), packet...)}
	manual, err := generate(ctx, input)
	if err != nil {
		return input, err
	}
	manual.ProjectID = id
	manual.SourcePacketDigest = p.MasterPacketDigest
	manual.ManualVersion = 1
	if err := s.SaveManual(id, manual); err != nil {
		return input, err
	}
	state.State = LifecycleActive
	state.Activities = []string{ActivityPlanning}
	state.CurrentPhase = "planning"
	state.LastEvent = "project intake resumed"
	state.Reason = "pending durable intake completed; ready for planning"
	state.UpdatedBy = "Yardmaster"
	if err := s.SaveState(id, state); err != nil {
		return ProjectContext{}, err
	}
	return s.LoadContext(id)
}

func (s Store) AmendAndIngest(ctx context.Context, id string, contents []byte, generate ManualGenerator) (ProjectContext, error) {
	if _, err := s.AddAmendment(id, contents); err != nil {
		return ProjectContext{}, err
	}
	input, err := s.LoadContext(id)
	if err != nil {
		return ProjectContext{}, err
	}
	if generate == nil {
		return input, fmt.Errorf("amendment persisted but no manual generator was supplied")
	}
	manual, err := generate(ctx, input)
	if err != nil {
		return input, err
	}
	manual.ProjectID = id
	manual.SourcePacketDigest = input.Project.MasterPacketDigest
	manual.ManualVersion = input.Project.ProjectManualVersion + 1
	if err := s.SaveManual(id, manual); err != nil {
		return input, err
	}
	return s.LoadContext(id)
}

func ReadPacketFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read master packet: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("master packet is empty")
	}
	return data, nil
}
