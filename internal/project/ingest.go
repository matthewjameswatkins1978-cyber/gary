package project

import (
	"context"
	"fmt"
	"os"
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
	input := ProjectContext{Project: p, State: ProjectState{Schema: StateSchema, ProjectID: id, State: "ACTIVE", CurrentPhase: "intake"}, Packet: append([]byte(nil), packet...)}
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
	if _, err := s.EnsureAcceptanceContract(id); err != nil {
		return input, err
	}
	state := input.State
	state.State = "PLANNING"
	state.CurrentPhase = "planning"
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
	if input.Project.AcceptanceContractVersion > 0 {
		contract := DeriveAcceptanceContract(input.Project, manual)
		contract.ContractVersion = input.Project.AcceptanceContractVersion + 1
		contract.ContractDigest = AcceptanceContractDigest(contract)
		if err := s.replaceAcceptanceContract(contract); err != nil {
			return input, err
		}
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
