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
	ProjectSchema   = "gary.project.v1"
	ManualSchema    = "gary.project-manual.v1"
	AmendmentSchema = "gary.project-amendment.v1"
	StateSchema     = "gary.project-state.v1"
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
	MasterPacketDigest   string `json:"master_packet_digest"`
	MasterPacketVersion  int    `json:"master_packet_version"`
	ProjectManualVersion int    `json:"project_manual_version"`
	ManualDigest         string `json:"manual_digest,omitempty"`
	CreatedAt            string `json:"created_at"`
	UpdatedAt            string `json:"updated_at"`
	AcceptedAt           string `json:"accepted_at,omitempty"`
}

// ProjectState separates transient progress from the durable project manual.
type ProjectState struct {
	Schema                string   `json:"schema"`
	ProjectID             string   `json:"project_id"`
	State                 string   `json:"state"`
	CurrentPhase          string   `json:"current_phase,omitempty"`
	CurrentStrategy       string   `json:"current_strategy,omitempty"`
	CompletedCapabilities []string `json:"completed_capabilities,omitempty"`
	RemainingCapabilities []string `json:"remaining_capabilities,omitempty"`
	OutstandingWork       []string `json:"outstanding_work,omitempty"`
	LastEvent             string   `json:"last_event,omitempty"`
	UpdatedAt             string   `json:"updated_at"`
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
	if err := os.WriteFile(filepath.Join(dir, "source", "MASTER_PACKET.md"), packet, 0o644); err != nil {
		return Project{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	p := Project{Schema: ProjectSchema, ProjectID: id, ProjectName: name, Repo: repo, BaseBranch: baseBranch, State: "ACTIVE", MasterPacketDigest: Digest(packet), MasterPacketVersion: 1, CreatedAt: now, UpdatedAt: now}
	state := ProjectState{Schema: StateSchema, ProjectID: id, State: "ACTIVE", CurrentPhase: "intake", UpdatedAt: now}
	if err := writeJSON(filepath.Join(dir, "project.json"), p); err != nil {
		return Project{}, err
	}
	if err := writeJSON(filepath.Join(dir, "state.json"), state); err != nil {
		return Project{}, err
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
	if p.Schema != ProjectSchema || p.ProjectID != id || p.MasterPacketVersion != 1 || p.MasterPacketDigest == "" {
		return Project{}, fmt.Errorf("%w: invalid project metadata", ErrProjectMissing)
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
	if state.Schema != StateSchema || state.ProjectID != id {
		return ProjectContext{}, fmt.Errorf("%w: invalid project state", ErrProjectMissing)
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
	if p.State == "ACCEPTED" || p.State == "ARCHIVED" {
		return fmt.Errorf("%w: project state is %s", ErrProjectNotAcceptable, p.State)
	}
	if manual.ProjectID != id || manual.SourcePacketDigest != p.MasterPacketDigest {
		return fmt.Errorf("%w: manual source or project mismatch", ErrProjectIDMismatch)
	}
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
	return writeJSON(filepath.Join(s.projectDir(id), "project.json"), p)
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
	if p.State == "ACCEPTED" || p.State == "ARCHIVED" {
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
	p, err := s.LoadProject(id)
	if err != nil {
		return err
	}
	if state.ProjectID != id || !validProjectState(state.State) {
		return fmt.Errorf("%w: invalid state", ErrProjectMissing)
	}
	if p.State == "ARCHIVED" && state.State != "ARCHIVED" {
		return fmt.Errorf("%w: terminal project state ARCHIVED cannot transition to %s", ErrProjectNotAcceptable, state.State)
	}
	if p.State == "ACCEPTED" && state.State != "ACCEPTED" && state.State != "ARCHIVED" {
		return fmt.Errorf("%w: terminal project state ACCEPTED cannot transition to %s", ErrProjectNotAcceptable, state.State)
	}
	state.Schema = StateSchema
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := writeJSON(filepath.Join(s.projectDir(id), "state.json"), state); err != nil {
		return err
	}
	p.State = state.State
	p.UpdatedAt = state.UpdatedAt
	return writeJSON(filepath.Join(s.projectDir(id), "project.json"), p)
}

func (s Store) AcceptProject(id, summary string) error {
	p, err := s.LoadProject(id)
	if err != nil {
		return err
	}
	if _, err := s.LoadContext(id); err != nil {
		return err
	}
	if p.State != "VERIFYING" {
		return fmt.Errorf("%w: state is %s", ErrProjectNotAcceptable, p.State)
	}
	state := ProjectState{Schema: StateSchema, ProjectID: id, State: "ACCEPTED", CurrentPhase: "accepted", LastEvent: summary}
	if err := s.SaveState(id, state); err != nil {
		return err
	}
	p, _ = s.LoadProject(id)
	p.AcceptedAt = time.Now().UTC().Format(time.RFC3339)
	return writeJSON(filepath.Join(s.projectDir(id), "project.json"), p)
}

// ResumeProject performs a fresh validated context load and moves a retained
// non-terminal project back to planning. It does not launch workers by itself.
func (s Store) ResumeProject(id string) (ProjectContext, error) {
	ctx, err := s.LoadContext(id)
	if err != nil {
		return ProjectContext{}, err
	}
	if ctx.Project.State == "ARCHIVED" || ctx.Project.State == "ACCEPTED" {
		return ProjectContext{}, fmt.Errorf("%w: project state is %s", ErrProjectNotAcceptable, ctx.Project.State)
	}
	state := ctx.State
	state.State = "PLANNING"
	state.CurrentPhase = "planning"
	state.LastEvent = "project resumed"
	if err := s.SaveState(id, state); err != nil {
		return ProjectContext{}, err
	}
	return s.LoadContext(id)
}

func (s Store) ArchiveProject(id string) error {
	p, err := s.LoadProject(id)
	if err != nil {
		return err
	}
	if p.State != "ACCEPTED" {
		return fmt.Errorf("%w: project state is %s", ErrProjectNotAcceptable, p.State)
	}
	state := ProjectState{Schema: StateSchema, ProjectID: id, State: "ARCHIVED", CurrentPhase: "archived", LastEvent: "project archived"}
	if err := s.SaveState(id, state); err != nil {
		return err
	}
	archive := filepath.Join(s.projectDir(id), "archive", "archive.json")
	return writeJSON(archive, map[string]string{"project_id": id, "archived_at": time.Now().UTC().Format(time.RFC3339)})
}

func ValidateManual(m ProjectManual, id, packetDigest string, version int) error {
	if m.Schema != ManualSchema || m.ProjectID != id || m.SourcePacketDigest != packetDigest || m.ManualVersion != version {
		return errors.New("schema, project, source digest, or version mismatch")
	}
	if strings.TrimSpace(m.Objective) == "" || strings.TrimSpace(m.DefinitionOfDone) == "" {
		return errors.New("objective and definition_of_done are required")
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
	switch state {
	case "ACTIVE", "PLANNING", "EXECUTING", "VERIFYING", "ACCEPTED", "ARCHIVED", "FAILED", "PAUSED":
		return true
	default:
		return false
	}
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
