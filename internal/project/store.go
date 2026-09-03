// Package project provides durable project-level context for GARY.
//
// The store is deliberately ordinary files: the original packet is immutable,
// manuals are versioned snapshots, amendments are append-only, and project
// state is kept separate from the manual and execution history.
package project

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	ProjectSchema       = "gary.project.v2"
	LegacyProjectSchema = "gary.project.v1"
	ManualSchema        = "gary.project-manual.v1"
	AmendmentSchema     = "gary.project-amendment.v1"
	StateSchema         = "gary.project-state.v2"
	LegacyStateSchema   = "gary.project-state.v1"
)

var projectIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,59}$`)

var (
	ErrProjectMissing       = errors.New("PROJECT_CONTEXT_MISSING")
	ErrPacketMissing        = errors.New("PROJECT_PACKET_MISSING")
	ErrPacketDigestMismatch = errors.New("PROJECT_PACKET_DIGEST_MISMATCH")
	ErrManualMissing        = errors.New("PROJECT_MANUAL_MISSING")
	ErrManualInvalid        = errors.New("PROJECT_MANUAL_INVALID")
	ErrProjectIDMismatch    = errors.New("PROJECT_ID_MISMATCH")
	ErrManualVersionInvalid = errors.New("PROJECT_MANUAL_VERSION_INVALID")
	ErrProjectNotAcceptable = errors.New("PROJECT_NOT_READY_FOR_ACCEPTANCE")
)

// IsValidID reports whether an ID is safe to use as a project directory name.
func IsValidID(id string) bool { return projectIDPattern.MatchString(id) }

// Project is the durable identity and current-version pointer for a project.
type Project struct {
	Schema               string `json:"schema"`
	ProjectID            string `json:"project_id"`
	ProjectName          string `json:"project_name,omitempty"`
	Repo                 string `json:"repo,omitempty"`
	BaseBranch           string `json:"base_branch,omitempty"`
	State                string `json:"state"`
	Generation           int    `json:"generation,omitempty"`
	MasterPacketDigest   string `json:"master_packet_digest"`
	MasterPacketVersion  int    `json:"master_packet_version"`
	ProjectManualVersion int    `json:"project_manual_version"`
	ManualDigest         string `json:"manual_digest,omitempty"`
	CreatedAt            string `json:"created_at"`
	UpdatedAt            string `json:"updated_at"`
	AcceptedAt           string `json:"accepted_at,omitempty"`
	CancelledAt          string `json:"cancelled_at,omitempty"`
	ArchivedAt           string `json:"archived_at,omitempty"`
}

// ProjectState separates transient progress from the durable project manual.
type ProjectState struct {
	Schema                string        `json:"schema"`
	ProjectID             string        `json:"project_id"`
	State                 string        `json:"state"`
	Generation            int           `json:"generation,omitempty"`
	Activities            []string      `json:"activities,omitempty"`
	YardmasterState       string        `json:"yardmaster_state,omitempty"`
	Completion            string        `json:"completion,omitempty"`
	ParkReason            string        `json:"park_reason,omitempty"`
	WaitingFor            string        `json:"waiting_for,omitempty"`
	ExternalDependency    string        `json:"external_dependency,omitempty"`
	BlockedUntil          string        `json:"blocked_until,omitempty"`
	Blocker               string        `json:"blocker,omitempty"`
	CurrentPhase          string        `json:"current_phase,omitempty"`
	CurrentStrategy       string        `json:"current_strategy,omitempty"`
	CompletedCapabilities []string      `json:"completed_capabilities,omitempty"`
	RemainingCapabilities []string      `json:"remaining_capabilities,omitempty"`
	OutstandingWork       []string      `json:"outstanding_work,omitempty"`
	Watchdog              WatchdogState `json:"watchdog,omitempty"`
	LastEvent             string        `json:"last_event,omitempty"`
	LastEventID           string        `json:"last_event_id,omitempty"`
	Reason                string        `json:"reason,omitempty"`
	UpdatedBy             string        `json:"updated_by,omitempty"`
	UpdatedAt             string        `json:"updated_at"`
}

type SourceRef struct {
	PacketDigest string `json:"packet_digest"`
	Locator      string `json:"locator"`
}

type Requirement struct {
	ID              string    `json:"id"`
	Type            string    `json:"type,omitempty"`
	Text            string    `json:"text"`
	Description     string    `json:"description,omitempty"`
	Decision        string    `json:"decision,omitempty"`
	Choice          string    `json:"choice,omitempty"`
	Command         string    `json:"command,omitempty"`
	ExpectedOutcome string    `json:"expected_outcome,omitempty"`
	Mitigation      string    `json:"mitigation,omitempty"`
	Status          string    `json:"status,omitempty"`
	Source          SourceRef `json:"source,omitempty"`
}

type Conflict struct {
	ID      string    `json:"id"`
	Subject string    `json:"subject"`
	Options []string  `json:"options"`
	Status  string    `json:"status"`
	Source  SourceRef `json:"source,omitempty"`
}

// ProjectManual is a structured working model, not an opaque prose summary.
type ProjectManual struct {
	Schema                    string        `json:"schema"`
	ProjectID                 string        `json:"project_id"`
	ProjectName               string        `json:"project_name,omitempty"`
	Objective                 string        `json:"objective"`
	DefinitionOfDone          string        `json:"definition_of_done"`
	HardConstraints           []Requirement `json:"hard_constraints,omitempty"`
	ArchitectureDecisions     []Requirement `json:"architecture_decisions,omitempty"`
	TechnologyChoices         []Requirement `json:"technology_choices,omitempty"`
	ForbiddenChanges          []Requirement `json:"forbidden_changes,omitempty"`
	FunctionalRequirements    []Requirement `json:"functional_requirements,omitempty"`
	NonFunctionalRequirements []Requirement `json:"non_functional_requirements,omitempty"`
	AcceptanceCriteria        []Requirement `json:"acceptance_criteria,omitempty"`
	ImportantContext          []Requirement `json:"important_context,omitempty"`
	ImportantRationale        []Requirement `json:"important_rationale,omitempty"`
	KnownDependencies         []Requirement `json:"known_dependencies,omitempty"`
	KnownExternalServices     []Requirement `json:"known_external_services,omitempty"`
	KnownRisks                []Requirement `json:"known_risks,omitempty"`
	OpenQuestions             []Requirement `json:"open_questions,omitempty"`
	Conflicts                 []Conflict    `json:"conflicts,omitempty"`
	CurrentPhase              string        `json:"current_phase,omitempty"`
	CurrentStrategy           string        `json:"current_strategy,omitempty"`
	CompletedCapabilities     []string      `json:"completed_capabilities,omitempty"`
	RemainingCapabilities     []string      `json:"remaining_capabilities,omitempty"`
	ActiveDecisions           []Requirement `json:"active_decisions,omitempty"`
	SupersededDecisions       []Requirement `json:"superseded_decisions,omitempty"`
	BoardPolicy               BoardPolicy   `json:"board_policy"`
	SourcePacketDigest        string        `json:"source_packet_digest"`
	ManualVersion             int           `json:"manual_version"`
	ManualDigest              string        `json:"manual_digest"`
}

type Amendment struct {
	Schema     string `json:"schema"`
	ID         string `json:"id"`
	Path       string `json:"path"`
	Digest     string `json:"digest"`
	ReceivedAt string `json:"received_at"`
	Contents   []byte `json:"-"`
}

type ProjectContext struct {
	Project         Project
	State           ProjectState
	Manual          ProjectManual
	Amendments      []Amendment
	Feedback        []FeedbackRecord
	AppliedFeedback []FeedbackRecord
	Packet          []byte
}

type Store struct{ root string }

func NewStore(root string) Store { return Store{root: root} }

func DefaultStore() Store {
	if root := os.Getenv("GARY_PROJECTS_DIR"); root != "" {
		return NewStore(root)
	}
	return NewStore(filepath.Join("gary", "projects"))
}

func (s Store) Root() string { return s.root }

func (s Store) projectDir(id string) string { return filepath.Join(s.root, id) }

func (s Store) CreateProject(id, name, repo, baseBranch string, packet []byte) (Project, error) {
	if !IsValidID(id) {
		return Project{}, fmt.Errorf("invalid project_id %q", id)
	}
	if len(packet) == 0 {
		return Project{}, errors.New("master packet is empty")
	}
	dir := s.projectDir(id)
	if _, err := os.Stat(dir); err == nil {
		return Project{}, fmt.Errorf("project %q already exists", id)
	} else if !errors.Is(err, os.ErrNotExist) {
		return Project{}, err
	}
	if err := os.MkdirAll(filepath.Join(dir, "manuals"), 0o755); err != nil {
		return Project{}, err
	}
	if err := os.MkdirAll(filepath.Join(dir, "source"), 0o755); err != nil {
		return Project{}, err
	}
	if err := os.MkdirAll(filepath.Join(dir, "amendments"), 0o755); err != nil {
		return Project{}, err
	}
	if err := os.MkdirAll(filepath.Join(dir, "jobs"), 0o755); err != nil {
		return Project{}, err
	}
	if err := os.MkdirAll(filepath.Join(dir, "evidence"), 0o755); err != nil {
		return Project{}, err
	}
	if err := os.MkdirAll(filepath.Join(dir, "feedback"), 0o755); err != nil {
		return Project{}, err
	}
	if err := os.MkdirAll(filepath.Join(dir, "events"), 0o755); err != nil {
		return Project{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, "source", "MASTER_PACKET.md"), packet, 0o644); err != nil {
		return Project{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	p := Project{Schema: ProjectSchema, ProjectID: id, ProjectName: name, Repo: repo, BaseBranch: baseBranch, State: LifecycleActive, Generation: 1, MasterPacketDigest: Digest(packet), MasterPacketVersion: 1, CreatedAt: now, UpdatedAt: now}
	state := ProjectState{Schema: StateSchema, ProjectID: id, State: LifecycleActive, Generation: 1, Activities: []string{ActivityPlanning}, CurrentPhase: "intake", LastEvent: "project created", LastEventID: fmt.Sprintf("%s-lifecycle-%06d", id, 1), Reason: "project created", UpdatedBy: "control-plane", UpdatedAt: now}
	if err := writeJSON(filepath.Join(dir, "project.json"), p); err != nil {
		return Project{}, err
	}
	if err := writeJSON(filepath.Join(dir, "state.json"), state); err != nil {
		return Project{}, err
	}
	if err := s.initialiseLifecycleEvent(id, now); err != nil {
		return Project{}, err
	}
	if err := s.MarkProjectInProcess(p); err != nil {
		return Project{}, fmt.Errorf("project persisted but board update failed: %w", err)
	}
	return p, nil
}

func (s Store) LoadProject(id string) (Project, error) {
	if !IsValidID(id) {
		return Project{}, fmt.Errorf("invalid project_id %q", id)
	}
	var p Project
	if err := readJSON(filepath.Join(s.projectDir(id), "project.json"), &p); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Project{}, fmt.Errorf("%w: %s", ErrProjectMissing, id)
		}
		return Project{}, err
	}
	if (p.Schema != ProjectSchema && p.Schema != LegacyProjectSchema) || p.ProjectID != id || p.MasterPacketVersion != 1 || p.MasterPacketDigest == "" {
		return Project{}, fmt.Errorf("%w: invalid project metadata", ErrProjectMissing)
	}
	p.Schema = ProjectSchema
	lifecycle, _, archived := canonicalLifecycle(p.State)
	if lifecycle == "" {
		return Project{}, fmt.Errorf("%w: invalid project lifecycle %q", ErrProjectMissing, p.State)
	}
	p.State = lifecycle
	if p.Generation < 1 {
		p.Generation = 1
	}
	if archived && p.ArchivedAt == "" {
		p.ArchivedAt = p.UpdatedAt
	}
	return p, nil
}

func (s Store) LoadMasterPacket(id string) ([]byte, error) {
	p, err := s.LoadProject(id)
	if err != nil {
		return nil, err
	}
	packet, err := os.ReadFile(filepath.Join(s.projectDir(id), "source", "MASTER_PACKET.md"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", ErrPacketMissing, id)
	}
	if err != nil {
		return nil, err
	}
	if Digest(packet) != p.MasterPacketDigest {
		return nil, fmt.Errorf("%w: %s", ErrPacketDigestMismatch, id)
	}
	return packet, nil
}

func (s Store) LoadCurrentManual(id string) (ProjectManual, error) {
	p, err := s.LoadProject(id)
	if err != nil {
		return ProjectManual{}, err
	}
	if p.ProjectManualVersion < 1 || p.ManualDigest == "" {
		return ProjectManual{}, fmt.Errorf("%w: %s", ErrManualMissing, id)
	}
	path := filepath.Join(s.projectDir(id), "manuals", fmt.Sprintf("project-manual-v%03d.json", p.ProjectManualVersion))
	var m ProjectManual
	if err := readJSON(path, &m); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ProjectManual{}, fmt.Errorf("%w: %s", ErrManualMissing, id)
		}
		return ProjectManual{}, fmt.Errorf("%w: %v", ErrManualInvalid, err)
	}
	if err := ValidateManual(m, id, p.MasterPacketDigest, p.ProjectManualVersion); err != nil {
		return ProjectManual{}, fmt.Errorf("%w: %v", ErrManualInvalid, err)
	}
	if m.ManualDigest != p.ManualDigest || ManualDigest(m) != m.ManualDigest {
		return ProjectManual{}, fmt.Errorf("%w: digest mismatch", ErrManualInvalid)
	}
	return m, nil
}

func (s Store) LoadContext(id string) (ProjectContext, error) {
	p, err := s.LoadProject(id)
	if err != nil {
		return ProjectContext{}, err
	}
	packet, err := s.LoadMasterPacket(id)
	if err != nil {
		return ProjectContext{}, err
	}
	manual, err := s.LoadCurrentManual(id)
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
	state, archived, err := normalizeProjectedState(state)
	if err != nil {
		return ProjectContext{}, err
	}
	if state.Generation < 1 {
		state.Generation = p.Generation
	}
	if archived && p.ArchivedAt == "" {
		p.ArchivedAt = p.UpdatedAt
	}
	amendments, err := s.LoadAmendments(id)
	if err != nil {
		return ProjectContext{}, err
	}
	feedback, err := s.LoadFeedback(id)
	if err != nil {
		return ProjectContext{}, err
	}
	var applied []FeedbackRecord
	for _, record := range feedback {
		if record.Status == FeedbackApplied {
			applied = append(applied, record)
		}
	}
	return ProjectContext{Project: p, State: state, Manual: manual, Amendments: amendments, Feedback: feedback, AppliedFeedback: applied, Packet: packet}, nil
}

func (s Store) SaveManual(id string, manual ProjectManual) error {
	p, err := s.LoadProject(id)
	if err != nil {
		return err
	}
	if p.State == LifecycleAccepted || p.State == LifecycleCancelled {
		return fmt.Errorf("%w: project state is %s", ErrProjectNotAcceptable, p.State)
	}
	if manual.ProjectID != id || manual.SourcePacketDigest != p.MasterPacketDigest {
		return fmt.Errorf("%w: manual source or project mismatch", ErrProjectIDMismatch)
	}
	// Board lifecycle is a GARY-owned invariant, not a model-controlled
	// project choice. Preserve a valid condition, but repair omitted or
	// partial model output before calculating the durable manual digest.
	manual.BoardPolicy = NormalizeBoardPolicy(manual.BoardPolicy)
	wantVersion := p.ProjectManualVersion + 1
	if manual.ManualVersion != wantVersion {
		return fmt.Errorf("%w: got %d, want %d", ErrManualVersionInvalid, manual.ManualVersion, wantVersion)
	}
	manual.Schema = ManualSchema
	manual.ManualDigest = ManualDigest(manual)
	if err := ValidateManual(manual, id, p.MasterPacketDigest, wantVersion); err != nil {
		return fmt.Errorf("%w: %v", ErrManualInvalid, err)
	}
	path := filepath.Join(s.projectDir(id), "manuals", fmt.Sprintf("project-manual-v%03d.json", wantVersion))
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("manual version %d already exists", wantVersion)
	}
	if err := writeJSON(path, manual); err != nil {
		return err
	}
	p.ProjectManualVersion = wantVersion
	p.ManualDigest = manual.ManualDigest
	p.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := writeJSON(filepath.Join(s.projectDir(id), "project.json"), p); err != nil {
		return err
	}
	return s.MarkProjectInProcess(p)
}

func (s Store) LoadAmendments(id string) ([]Amendment, error) {
	if _, err := s.LoadProject(id); err != nil {
		return nil, err
	}
	dir := filepath.Join(s.projectDir(id), "amendments")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Amendment
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		var a Amendment
		if err := readJSON(filepath.Join(dir, entry.Name()), &a); err != nil {
			return nil, err
		}
		if a.Schema != AmendmentSchema || a.ID == "" || a.Digest == "" {
			return nil, fmt.Errorf("invalid amendment %s", entry.Name())
		}
		contents, err := os.ReadFile(filepath.Join(s.projectDir(id), filepath.FromSlash(a.Path)))
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: amendment source %s", ErrProjectMissing, a.ID)
		}
		if err != nil {
			return nil, err
		}
		if Digest(contents) != a.Digest {
			return nil, fmt.Errorf("%w: amendment %s", ErrPacketDigestMismatch, a.ID)
		}
		a.Contents = contents
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s Store) AddAmendment(id string, contents []byte) (Amendment, error) {
	if len(contents) == 0 {
		return Amendment{}, errors.New("amendment is empty")
	}
	p, err := s.LoadProject(id)
	if err != nil {
		return Amendment{}, err
	}
	if p.State == LifecycleAccepted || p.State == LifecycleCancelled {
		return Amendment{}, fmt.Errorf("%w: project state is %s", ErrProjectNotAcceptable, p.State)
	}
	amendments, err := s.LoadAmendments(id)
	if err != nil {
		return Amendment{}, err
	}
	n := len(amendments) + 1
	amendment := Amendment{Schema: AmendmentSchema, ID: fmt.Sprintf("amendment-%03d", n), Path: filepath.ToSlash(filepath.Join("amendments", fmt.Sprintf("amendment-%03d.md", n))), Digest: Digest(contents), ReceivedAt: time.Now().UTC().Format(time.RFC3339)}
	dir := filepath.Join(s.projectDir(id), "amendments")
	if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("amendment-%03d.md", n)), contents, 0o644); err != nil {
		return Amendment{}, err
	}
	if err := writeJSON(filepath.Join(dir, fmt.Sprintf("amendment-%03d.json", n)), amendment); err != nil {
		return Amendment{}, err
	}
	return amendment, nil
}

func (s Store) SaveState(id string, state ProjectState) error {
	return s.withProjectStateLock(id, func() error {
		return s.saveStateUnlocked(id, state)
	})
}

func (s Store) saveStateUnlocked(id string, state ProjectState) error {
	p, err := s.LoadProject(id)
	if err != nil {
		return err
	}
	if state.ProjectID != id {
		return fmt.Errorf("%w: invalid project id in state", ErrProjectMissing)
	}

	var prior ProjectState
	if err := readJSON(filepath.Join(s.projectDir(id), "state.json"), &prior); err != nil {
		return fmt.Errorf("%w: state: %v", ErrProjectMissing, err)
	}
	prior, _, err = normalizeProjectedState(prior)
	if err != nil {
		return err
	}
	if prior.Generation < 1 {
		prior.Generation = p.Generation
	}

	state, _, err = normalizeProjectedState(state)
	if err != nil {
		return err
	}
	lifecycleTransition := state.State != p.State
	if lifecycleTransition && state.Generation == 0 {
		return fmt.Errorf("%w: lifecycle transition %s -> %s requires expected generation %d", ErrLifecycleStaleGeneration, p.State, state.State, p.Generation)
	}
	if state.Generation != 0 && state.Generation != p.Generation {
		return fmt.Errorf("%w: got %d, current %d", ErrLifecycleStaleGeneration, state.Generation, p.Generation)
	}
	if p.State == LifecycleAccepted || p.State == LifecycleCancelled {
		if state.State != p.State {
			return fmt.Errorf("%w: terminal project state %s cannot transition to %s", ErrProjectNotAcceptable, p.State, state.State)
		}
	}

	if state.YardmasterState == "" {
		state.YardmasterState = prior.YardmasterState
	}
	if state.Completion == "" {
		state.Completion = prior.Completion
	}
	if state.ParkReason == "" {
		state.ParkReason = prior.ParkReason
	}
	if state.Watchdog == (WatchdogState{}) {
		state.Watchdog = prior.Watchdog
	}
	if state.WaitingFor == "" && state.State == LifecycleWaiting {
		state.WaitingFor = prior.WaitingFor
	}
	if state.Blocker == "" && state.State == LifecycleBlocked {
		state.Blocker = prior.Blocker
	}
	if state.State == LifecycleWaiting && strings.TrimSpace(state.WaitingFor) == "" {
		return fmt.Errorf("%w: WAITING requires waiting_for", ErrProjectNotAcceptable)
	}
	if state.State == LifecycleBlocked && strings.TrimSpace(state.Blocker) == "" {
		return fmt.Errorf("%w: BLOCKED requires blocker", ErrProjectNotAcceptable)
	}
	if state.UpdatedBy == "" {
		state.UpdatedBy = "control-plane"
	}
	if state.Reason == "" {
		state.Reason = strings.TrimSpace(state.LastEvent)
	}
	if state.Reason == "" {
		state.Reason = "project state projection updated"
	}

	state = state.normalized(state.State)
	state.Schema = StateSchema
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)

	if state.State != p.State {
		if !legalLifecycleTransition(p.State, state.State) {
			return fmt.Errorf("%w: illegal lifecycle transition %s -> %s", ErrProjectNotAcceptable, p.State, state.State)
		}
		if err := s.ensureLifecycleHistoryUnlocked(p); err != nil {
			return err
		}
		nextGeneration := p.Generation + 1
		eventID := fmt.Sprintf("%s-lifecycle-%06d", id, nextGeneration)
		event := LifecycleEvent{
			ProjectID: id, EventID: eventID, Generation: nextGeneration,
			PreviousState: p.State, NewState: state.State,
			Actor: state.UpdatedBy, Reason: state.Reason,
			CreatedAt: state.UpdatedAt,
		}
		if err := s.appendLifecycleEventUnlocked(event); err != nil {
			return err
		}
		p.Generation = nextGeneration
		state.Generation = nextGeneration
		state.LastEventID = eventID
		if state.State == LifecycleAccepted && p.AcceptedAt == "" {
			p.AcceptedAt = state.UpdatedAt
		}
		if state.State == LifecycleCancelled && p.CancelledAt == "" {
			p.CancelledAt = state.UpdatedAt
		}
	} else {
		state.Generation = p.Generation
		if state.LastEventID == "" {
			state.LastEventID = prior.LastEventID
		}
	}

	if err := writeJSON(filepath.Join(s.projectDir(id), "state.json"), state); err != nil {
		return err
	}
	p.State = state.State
	p.UpdatedAt = state.UpdatedAt
	return writeJSON(filepath.Join(s.projectDir(id), "project.json"), p)
}

func (s Store) AcceptProject(id, summary string) error {
	ctx, err := s.LoadContext(id)
	if err != nil {
		return err
	}
	if ctx.Project.State != LifecycleActive {
		return fmt.Errorf("%w: lifecycle state is %s", ErrProjectNotAcceptable, ctx.Project.State)
	}
	if !ctx.State.HasActivity(ActivityVerifying) && strings.ToLower(ctx.State.CurrentPhase) != "verification" {
		return fmt.Errorf("%w: project is not in verification activity", ErrProjectNotAcceptable)
	}
	state := ctx.State
	state.State = LifecycleAccepted
	state.Activities = nil
	state.CurrentPhase = "accepted"
	state.LastEvent = summary
	state.Reason = summary
	state.UpdatedBy = "Yardmaster"
	if err := s.SaveState(id, state); err != nil {
		return err
	}
	p, err := s.LoadProject(id)
	if err != nil {
		return err
	}
	return s.MarkProjectCompleted(p)
}

// ResumeProject performs a fresh validated context load and returns a
// resumable non-terminal project to ACTIVE planning. It does not launch workers.
func (s Store) ResumeProject(id string) (ProjectContext, error) {
	ctx, err := s.LoadContext(id)
	if err != nil {
		return ProjectContext{}, err
	}
	if ctx.Project.State == LifecycleAccepted || ctx.Project.State == LifecycleCancelled {
		return ProjectContext{}, fmt.Errorf("%w: project state is %s", ErrProjectNotAcceptable, ctx.Project.State)
	}
	state := ctx.State
	state.State = LifecycleActive
	state.Activities = []string{ActivityPlanning}
	state.CurrentPhase = "planning"
	state.YardmasterState = YardmasterRunning
	state.LastEvent = "project resumed"
	state.Reason = "project resumed"
	state.UpdatedBy = "Yardmaster"
	if err := s.SaveState(id, state); err != nil {
		return ProjectContext{}, err
	}
	p, _ := s.LoadProject(id)
	if err := s.MarkProjectInProcess(p); err != nil {
		return ProjectContext{}, err
	}
	return s.LoadContext(id)
}

// CancelProject explicitly closes an unfinished project without acceptance.
func (s Store) CancelProject(id, reason string) error {
	ctx, err := s.LoadContext(id)
	if err != nil {
		return err
	}
	if ctx.Project.State == LifecycleAccepted || ctx.Project.State == LifecycleCancelled {
		return fmt.Errorf("%w: project state is %s", ErrProjectNotAcceptable, ctx.Project.State)
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "project cancelled by authorised control action"
	}
	state := ctx.State
	state.State = LifecycleCancelled
	state.Activities = nil
	state.CurrentPhase = "cancelled"
	state.LastEvent = reason
	state.Reason = reason
	state.UpdatedBy = "control-plane"
	if err := s.SaveState(id, state); err != nil {
		return err
	}
	p, err := s.LoadProject(id)
	if err != nil {
		return err
	}
	return s.MarkProjectCancelled(p)
}

// ArchiveProject is storage metadata only. It does not change lifecycle.
func (s Store) ArchiveProject(id string) error {
	p, err := s.LoadProject(id)
	if err != nil {
		return err
	}
	if p.State != LifecycleAccepted && p.State != LifecycleCancelled {
		return fmt.Errorf("%w: project state is %s", ErrProjectNotAcceptable, p.State)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	p.ArchivedAt = now
	p.UpdatedAt = now
	if err := writeJSON(filepath.Join(s.projectDir(id), "project.json"), p); err != nil {
		return err
	}
	archive := filepath.Join(s.projectDir(id), "archive", "archive.json")
	return writeJSON(archive, map[string]string{"project_id": id, "lifecycle_state": p.State, "archived_at": now})
}

func ValidateManual(m ProjectManual, id, packetDigest string, version int) error {
	if m.Schema != ManualSchema || m.ProjectID != id || m.SourcePacketDigest != packetDigest || m.ManualVersion != version {
		return errors.New("schema, project, source digest, or version mismatch")
	}
	if strings.TrimSpace(m.Objective) == "" || strings.TrimSpace(m.DefinitionOfDone) == "" {
		return errors.New("objective and definition_of_done are required")
	}
	if !m.BoardPolicy.isZero() && !m.BoardPolicy.valid() {
		return errors.New("board_policy must define Yardmaster-owned IN_PROCESS to COMPLETED lifecycle")
	}
	for _, r := range allRequirements(m) {
		if strings.TrimSpace(r.ID) == "" || strings.TrimSpace(r.Text) == "" {
			return errors.New("manual requirement has empty id or text")
		}
		if r.Source.PacketDigest != packetDigest || strings.TrimSpace(r.Source.Locator) == "" {
			return fmt.Errorf("requirement %s lacks source traceability", r.ID)
		}
		if r.Status != "" && !validEvidenceStatus(r.Status) {
			return fmt.Errorf("requirement %s has invalid status %q", r.ID, r.Status)
		}
		if r.Source.PacketDigest != "" && r.Source.PacketDigest != packetDigest {
			return fmt.Errorf("requirement %s has wrong packet digest", r.ID)
		}
	}
	for _, c := range m.Conflicts {
		if strings.TrimSpace(c.ID) == "" || strings.TrimSpace(c.Subject) == "" || len(c.Options) < 2 {
			return errors.New("manual conflict is incomplete")
		}
		if c.Status != "" && !validEvidenceStatus(c.Status) {
			return fmt.Errorf("conflict %s has invalid status %q", c.ID, c.Status)
		}
		if c.Source.PacketDigest != packetDigest || strings.TrimSpace(c.Source.Locator) == "" {
			return fmt.Errorf("conflict %s lacks source traceability", c.ID)
		}
	}
	return nil
}

func validEvidenceStatus(status string) bool {
	switch status {
	case "KNOWN", "INFERRED", "UNRESOLVED", "CONTRADICTORY":
		return true
	default:
		return false
	}
}

func allRequirements(m ProjectManual) []Requirement {
	out := make([]Requirement, 0)
	out = append(out, m.HardConstraints...)
	out = append(out, m.ArchitectureDecisions...)
	out = append(out, m.TechnologyChoices...)
	out = append(out, m.ForbiddenChanges...)
	out = append(out, m.FunctionalRequirements...)
	out = append(out, m.NonFunctionalRequirements...)
	out = append(out, m.AcceptanceCriteria...)
	out = append(out, m.ImportantContext...)
	out = append(out, m.ImportantRationale...)
	out = append(out, m.KnownDependencies...)
	out = append(out, m.KnownExternalServices...)
	out = append(out, m.KnownRisks...)
	out = append(out, m.OpenQuestions...)
	out = append(out, m.ActiveDecisions...)
	out = append(out, m.SupersededDecisions...)
	return out
}

func Digest(data []byte) string {
	h := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(h[:])
}

func ManualDigest(m ProjectManual) string {
	m.ManualDigest = ""
	b, _ := json.Marshal(m)
	return Digest(b)
}

func validProjectState(state string) bool {
	lifecycle, _, _ := canonicalLifecycle(state)
	return lifecycle != ""
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".gary-project-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func readJSON(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return errors.New("more than one JSON value")
	}
	return nil
}

// ManualVersionName is used by callers that need a stable file label.
func ManualVersionName(version int) string { return "project-manual-v" + strconv.Itoa(version) }
