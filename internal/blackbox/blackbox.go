// Package blackbox provides GARY's local operational memory. Raw job state and
// event journals remain authoritative; this package derives notes and a
// searchable index from them and stores diagnostic records separately.
package blackbox

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const SchemaVersion = 1

type Event struct {
	EventID   string         `json:"event_id"`
	JobID     string         `json:"job_id"`
	Type      string         `json:"type"`
	WorkerID  string         `json:"worker_id,omitempty"`
	Timestamp string         `json:"timestamp"`
	Data      map[string]any `json:"data,omitempty"`
}

type Worker struct {
	ID             string `json:"id"`
	Profile        string `json:"profile"`
	Status         string `json:"status"`
	Worktree       string `json:"worktree,omitempty"`
	Branch         string `json:"branch,omitempty"`
	BaseSHA        string `json:"base_sha,omitempty"`
	CommitSHA      string `json:"commit_sha,omitempty"`
	StartedAt      string `json:"started_at,omitempty"`
	FinishedAt     string `json:"finished_at,omitempty"`
	LogPath        string `json:"log_path,omitempty"`
	AcceptancePath string `json:"acceptance_path,omitempty"`
	PID            int    `json:"pid,omitempty"`
	ProcessAlive   bool   `json:"process_alive"`
}

type Acceptance struct {
	WorkerID string `json:"worker_id"`
	Passed   bool   `json:"passed"`
	Recorded string `json:"recorded_at,omitempty"`
}

type Relationship struct {
	SchemaVersion int    `json:"schema_version"`
	FromJobID     string `json:"from_job_id"`
	ToJobID       string `json:"to_job_id"`
	Kind          string `json:"kind"`
	Reason        string `json:"reason"`
	Evidence      string `json:"evidence"`
	CreatedAt     string `json:"created_at"`
}

type Incident struct {
	SchemaVersion      int      `json:"schema_version"`
	ID                 string   `json:"id"`
	Class              string   `json:"class"`
	Status             string   `json:"status"`
	JobIDs             []string `json:"job_ids,omitempty"`
	Symptom            string   `json:"symptom"`
	Expected           string   `json:"expected"`
	SuspectedCause     string   `json:"suspected_cause,omitempty"`
	ProvenCause        string   `json:"proven_cause,omitempty"`
	Recovery           string   `json:"recovery,omitempty"`
	Fix                string   `json:"fix,omitempty"`
	RegressionProof    string   `json:"regression_proof,omitempty"`
	RecurrenceCount    int      `json:"recurrence_count"`
	SimilarIncidentIDs []string `json:"similar_incident_ids,omitempty"`
	OpenedAt           string   `json:"opened_at"`
	UpdatedAt          string   `json:"updated_at"`
	ClosedAt           string   `json:"closed_at,omitempty"`
}

type Lesson struct {
	SchemaVersion int      `json:"schema_version"`
	ID            string   `json:"id"`
	Statement     string   `json:"statement"`
	Confidence    string   `json:"confidence"`
	Evidence      []string `json:"evidence"`
	JobIDs        []string `json:"job_ids,omitempty"`
	IncidentIDs   []string `json:"incident_ids,omitempty"`
	Destination   string   `json:"destination"` // code_fix, regression_test, operational_rule
	Status        string   `json:"status"`      // candidate, promoted, rejected
	CreatedAt     string   `json:"created_at"`
}

type HistoryRecord struct {
	SchemaVersion        int            `json:"schema_version"`
	JobID                string         `json:"job_id"`
	Title                string         `json:"title"`
	Task                 string         `json:"task,omitempty"`
	Repo                 string         `json:"repo"`
	Project              string         `json:"project,omitempty"`
	StartedAt            string         `json:"started_at,omitempty"`
	FinishedAt           string         `json:"finished_at,omitempty"`
	UpdatedAt            string         `json:"updated_at,omitempty"`
	Status               string         `json:"status"`
	AcceptanceCommands   []string       `json:"acceptance_commands,omitempty"`
	Workers              []Worker       `json:"workers,omitempty"`
	BaseCommit           string         `json:"base_commit,omitempty"`
	ResultCommits        []string       `json:"result_commits,omitempty"`
	Acceptance           []Acceptance   `json:"acceptance,omitempty"`
	FailureCategories    []string       `json:"failure_categories,omitempty"`
	Decisions            []string       `json:"decisions,omitempty"`
	MatthewInterventions int            `json:"matthew_interventions"`
	Lessons              []Lesson       `json:"lessons,omitempty"`
	NextAction           string         `json:"next_action"`
	LastAction           string         `json:"last_action"`
	PacketPath           string         `json:"packet_path,omitempty"`
	JournalPath          string         `json:"journal_path"`
	StatePath            string         `json:"state_path"`
	Artifacts            []string       `json:"artifacts,omitempty"`
	Tags                 []string       `json:"tags,omitempty"`
	RelatedJobs          []Relationship `json:"related_jobs,omitempty"`
	EventCount           int            `json:"event_count"`
}

type historyFile struct {
	SchemaVersion int             `json:"schema_version"`
	GeneratedAt   string          `json:"generated_at"`
	Records       []HistoryRecord `json:"records"`
}

type stateFile struct {
	Version              int    `json:"version"`
	Status               string `json:"status"`
	CreatedAt            string `json:"created_at"`
	UpdatedAt            string `json:"updated_at"`
	ThreadID             string `json:"thread_id"`
	MatthewInterventions int    `json:"matthew_interventions"`
	Blocker              *struct {
		Summary       string `json:"summary"`
		Needed        string `json:"needed"`
		Evidence      string `json:"evidence"`
		UnblockAction string `json:"unblock_action"`
		ResolvedAt    string `json:"resolved_at"`
	} `json:"blocker"`
	Job struct {
		ID            string   `json:"id"`
		RepoPath      string   `json:"repo_path"`
		BaseRef       string   `json:"base_ref"`
		Task          string   `json:"task"`
		Acceptance    []string `json:"acceptance"`
		WorkerProfile string   `json:"worker_profile"`
		Preflight     string   `json:"preflight"`
	} `json:"job"`
	Worker  *Worker  `json:"worker"`
	Workers []Worker `json:"workers"`
}

type Store struct {
	Root string
}

func New(root string) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("GARY state root is required")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(absolute, "blackbox"), 0o700); err != nil {
		return nil, fmt.Errorf("create Black Box directory: %w", err)
	}
	return &Store{Root: absolute}, nil
}

func (s *Store) blackboxDir() string  { return filepath.Join(s.Root, "blackbox") }
func (s *Store) historyPath() string  { return filepath.Join(s.blackboxDir(), "history.json") }
func (s *Store) incidentPath() string { return filepath.Join(s.blackboxDir(), "incidents.jsonl") }
func (s *Store) relationPath() string { return filepath.Join(s.blackboxDir(), "relations.jsonl") }
func (s *Store) lessonPath() string   { return filepath.Join(s.blackboxDir(), "lessons.jsonl") }

func safeName(value string, max int) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-_")
	if out == "" {
		out = "job"
	}
	if len(out) > max {
		out = strings.TrimRight(out[:max], "-_")
	}
	return out
}

func (s *Store) stateFiles() ([]string, error) {
	files, err := filepath.Glob(filepath.Join(s.Root, "jobs", "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func (s *Store) loadState(path string) (stateFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return stateFile{}, err
	}
	var state stateFile
	if err := json.Unmarshal(data, &state); err != nil {
		return stateFile{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if state.Job.ID == "" {
		return stateFile{}, errors.New("not a GARY state file")
	}
	return state, nil
}

func readEvents(path string) ([]Event, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var events []Event
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			// A torn final line must not make older evidence unreadable.
			continue
		}
		if event.JobID != "" && event.Type != "" {
			events = append(events, event)
		}
	}
	return events, scanner.Err()
}

func (s *Store) findState(jobID string) (string, stateFile, error) {
	for _, path := range mustStateFiles(s.stateFiles()) {
		state, err := s.loadState(path)
		if err == nil && state.Job.ID == jobID {
			return path, state, nil
		}
	}
	return "", stateFile{}, fmt.Errorf("job %q not found", jobID)
}

func mustStateFiles(files []string, err error) []string {
	if err != nil {
		return nil
	}
	return files
}

func (s *Store) BuildRecord(path string, state stateFile) (HistoryRecord, error) {
	safe := safeName(state.Job.ID, 100)
	journal := filepath.Join(s.Root, "jobs", safe+".events.jsonl")
	events, err := readEvents(journal)
	if err != nil {
		return HistoryRecord{}, err
	}
	workers := state.Workers
	if len(workers) == 0 && state.Worker != nil {
		workers = []Worker{*state.Worker}
	}
	for i := range workers {
		workers[i] = redactWorker(workers[i])
		workers[i].ProcessAlive = processAlive(workers[i].PID)
	}
	record := HistoryRecord{
		SchemaVersion: SchemaVersion,
		JobID:         state.Job.ID, Title: redact(title(state.Job.Task, state.Job.ID)),
		Task: redact(state.Job.Task),
		Repo: redact(state.Job.RepoPath), Project: filepath.Base(filepath.Clean(state.Job.RepoPath)),
		StartedAt: state.CreatedAt, UpdatedAt: state.UpdatedAt, Status: state.Status, AcceptanceCommands: redactStrings(state.Job.Acceptance), Workers: workers,
		MatthewInterventions: state.MatthewInterventions, JournalPath: journal,
		StatePath: path, EventCount: len(events), NextAction: nextAction(state, workers),
		PacketPath: packetPath(s.Root, state.Job.ID),
	}
	for _, worker := range workers {
		if record.BaseCommit == "" {
			record.BaseCommit = worker.BaseSHA
		}
		if worker.CommitSHA != "" && !contains(record.ResultCommits, worker.CommitSHA) {
			record.ResultCommits = append(record.ResultCommits, worker.CommitSHA)
		}
		if worker.LogPath != "" {
			record.Artifacts = append(record.Artifacts, worker.LogPath)
		}
		if worker.AcceptancePath != "" {
			record.Artifacts = append(record.Artifacts, worker.AcceptancePath)
		}
	}
	for _, event := range events {
		if event.Timestamp > record.FinishedAt && isTerminalEvent(event.Type) {
			record.FinishedAt = event.Timestamp
		}
		switch event.Type {
		case "JOB_CREATED", "PREFLIGHT_COMPLETED":
		case "ACCEPTANCE_COMPLETED":
			record.Acceptance = append(record.Acceptance, Acceptance{WorkerID: event.WorkerID, Passed: boolValue(event.Data, "passed"), Recorded: event.Timestamp})
			if !boolValue(event.Data, "passed") {
				record.FailureCategories = appendUnique(record.FailureCategories, "acceptance-failed")
			}
		case "WORKER_EXITED":
			if value, ok := event.Data["exit_code"]; ok && fmt.Sprint(value) != "0" {
				record.FailureCategories = appendUnique(record.FailureCategories, "worker-exit")
			}
			if boolValue(event.Data, "recovered") {
				record.Tags = appendUnique(record.Tags, "recovered-after-restart")
				if _, hasExitCode := event.Data["exit_code"]; !hasExitCode {
					record.FailureCategories = appendUnique(record.FailureCategories, "worker-interrupted")
				}
			}
		case "NEEDS_MATTHEW":
			record.FailureCategories = appendUnique(record.FailureCategories, "human-boundary")
		case "JOB_FAILED":
			record.FailureCategories = appendUnique(record.FailureCategories, "job-failed")
		case "SUPERVISOR_FAILED", "SUPERVISOR_LOST":
			record.FailureCategories = appendUnique(record.FailureCategories, "supervisor-failure")
		case "MATTHEW_RESUMED", "SMALL_FIX_APPLIED":
			record.Decisions = appendUnique(record.Decisions, event.Type)
		case "INSPECTION_COMPLETED":
			if commit := stringValue(event.Data, "commit_sha"); commit != "" && !contains(record.ResultCommits, commit) {
				record.ResultCommits = append(record.ResultCommits, commit)
			}
		case "OPERATOR_NOTE":
			if note := stringValue(event.Data, "text"); note != "" {
				record.Decisions = append(record.Decisions, note)
			}
		}
		if isMeaningfulEvent(event.Type) {
			record.LastAction = event.Type + eventDataHint(event.Data)
		}
	}
	if state.Blocker != nil && state.Status == "NEEDS_MATTHEW" {
		record.NextAction = state.Blocker.UnblockAction
	}
	record.FailureCategories = uniqueSorted(record.FailureCategories)
	record.Tags = uniqueSorted(record.Tags)
	record.Decisions = uniqueStrings(record.Decisions)
	record.RelatedJobs, _ = s.RelationshipsFor(state.Job.ID)
	record.Lessons, _ = s.LessonsFor(state.Job.ID)
	if record.Status != "RUNNING" && record.FinishedAt == "" {
		record.FinishedAt = state.UpdatedAt
	}
	return record, nil
}

func (s *Store) Rebuild() ([]HistoryRecord, error) {
	files, err := s.stateFiles()
	if err != nil {
		return nil, err
	}
	var records []HistoryRecord
	for _, path := range files {
		state, loadErr := s.loadState(path)
		if loadErr != nil {
			continue
		}
		record, buildErr := s.BuildRecord(path, state)
		if buildErr != nil {
			return nil, buildErr
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].StartedAt == records[j].StartedAt {
			return records[i].JobID < records[j].JobID
		}
		return records[i].StartedAt > records[j].StartedAt
	})
	file := historyFile{SchemaVersion: SchemaVersion, GeneratedAt: derivedAt(records), Records: records}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := atomicWrite(s.historyPath(), data, 0o600); err != nil {
		return nil, fmt.Errorf("write history index: %w", err)
	}
	return records, nil
}

func (s *Store) Records() ([]HistoryRecord, error) {
	// Rebuild on read keeps the catalogue current after a live job changes and
	// makes a corrupt/stale derived index harmless. The raw evidence is never
	// rewritten by this operation.
	return s.Rebuild()
}

func (s *Store) Record(jobID string) (HistoryRecord, error) {
	path, state, err := s.findState(jobID)
	if err != nil {
		return HistoryRecord{}, err
	}
	return s.BuildRecord(path, state)
}

func packetPath(root, jobID string) string {
	path := filepath.Join(root, "jobs", safeName(jobID, 100), "ORIGINAL_JOB.md")
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return ""
}

func notePath(root, jobID string) string {
	dir := filepath.Join(root, "jobs", safeName(jobID, 100))
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return filepath.Join(dir, "JOB_NOTE.md")
	}
	return filepath.Join(root, "jobs", safeName(jobID, 100)+".JOB_NOTE.md")
}

func (s *Store) RegenerateNote(jobID string) (string, error) {
	record, err := s.Record(jobID)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", record.Title)
	fmt.Fprintf(&b, "- Job: `%s`\n- Status: `%s`\n- Repository: `%s`\n- Started: `%s`\n- Finished: `%s`\n\n", record.JobID, record.Status, record.Repo, record.StartedAt, valueOrUnknown(record.FinishedAt))
	b.WriteString("## Goal\n\n")
	fmt.Fprintf(&b, "%s\n\n", record.Title)
	b.WriteString("## Where we are now\n\n")
	fmt.Fprintf(&b, "Last proven action: %s\n\nNext sensible action: %s\n\n", valueOrUnknown(record.LastAction), valueOrUnknown(record.NextAction))
	b.WriteString("## What happened\n\n")
	fmt.Fprintf(&b, "%d durable events recorded. Journal: `%s`\n\n", record.EventCount, record.JournalPath)
	if len(record.Workers) > 0 {
		for _, worker := range record.Workers {
			fmt.Fprintf(&b, "- Worker `%s` (%s): %s; branch `%s`; commit `%s`\n", worker.ID, worker.Profile, worker.Status, valueOrUnknown(worker.Branch), valueOrUnknown(worker.CommitSHA))
		}
		b.WriteString("\n")
	}
	b.WriteString("## What went wrong\n\n")
	if len(record.FailureCategories) == 0 {
		b.WriteString("None recorded.\n\n")
	} else {
		for _, item := range record.FailureCategories {
			fmt.Fprintf(&b, "- %s\n", item)
		}
		b.WriteString("\n")
	}
	b.WriteString("## Decisions / interventions\n\n")
	if len(record.Decisions) == 0 && record.MatthewInterventions == 0 {
		b.WriteString("None recorded.\n\n")
	} else {
		for _, item := range record.Decisions {
			fmt.Fprintf(&b, "- %s\n", item)
		}
		fmt.Fprintf(&b, "- Matthew interventions: %d\n\n", record.MatthewInterventions)
	}
	b.WriteString("## Current blocker\n\n")
	if record.Status == "NEEDS_MATTHEW" {
		fmt.Fprintf(&b, "%s\n\n", record.NextAction)
	} else {
		b.WriteString("None recorded.\n\n")
	}
	b.WriteString("## Important refs / evidence\n\n")
	fmt.Fprintf(&b, "- State: `%s`\n- Packet: `%s`\n", record.StatePath, valueOrUnknown(record.PacketPath))
	for _, artifact := range record.Artifacts {
		fmt.Fprintf(&b, "- Artifact: `%s`\n", artifact)
	}
	b.WriteString("\n")
	path := notePath(s.Root, jobID)
	if err := atomicWrite(path, []byte(b.String()), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Store) AppendJobEvent(jobID, typ string, data map[string]any) (Event, error) {
	path, _, err := s.findState(jobID)
	if err != nil {
		return Event{}, err
	}
	if strings.TrimSpace(typ) == "" {
		return Event{}, errors.New("event type is required")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	event := Event{EventID: "blackbox:" + safeName(jobID, 80) + ":" + strings.ToLower(typ) + ":" + now, JobID: jobID, Type: typ, Timestamp: now, Data: redactMap(data)}
	journal := filepath.Join(filepath.Dir(path), safeName(jobID, 100)+".events.jsonl")
	if err := appendUniqueJSONL(journal, event); err != nil {
		return Event{}, err
	}
	return event, nil
}

func (s *Store) AddRelationship(from, to, kind, reason, evidence string) (Relationship, error) {
	if strings.TrimSpace(from) == "" || strings.TrimSpace(to) == "" || strings.TrimSpace(kind) == "" || strings.TrimSpace(evidence) == "" {
		return Relationship{}, errors.New("from, to, kind and evidence are required")
	}
	if _, _, err := s.findState(from); err != nil {
		return Relationship{}, err
	}
	if _, _, err := s.findState(to); err != nil {
		return Relationship{}, err
	}
	rel := Relationship{SchemaVersion: SchemaVersion, FromJobID: from, ToJobID: to, Kind: kind, Reason: redact(reason), Evidence: redact(evidence), CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	return rel, appendUniqueJSONL(s.relationPath(), rel)
}

func (s *Store) RelationshipsFor(jobID string) ([]Relationship, error) {
	var all []Relationship
	if err := readJSONL(s.relationPath(), &all); err != nil {
		return nil, err
	}
	var out []Relationship
	for _, rel := range all {
		if rel.FromJobID == jobID || rel.ToJobID == jobID {
			out = append(out, rel)
		}
	}
	return out, nil
}

func (s *Store) OpenIncident(in Incident) (Incident, error) {
	if strings.TrimSpace(in.Class) == "" || strings.TrimSpace(in.Symptom) == "" || strings.TrimSpace(in.Expected) == "" {
		return Incident{}, errors.New("class, symptom and expected behavior are required")
	}
	for _, id := range in.JobIDs {
		if _, _, err := s.findState(id); err != nil {
			return Incident{}, err
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	in.SchemaVersion, in.Status, in.OpenedAt, in.UpdatedAt = SchemaVersion, "OPEN", now, now
	digest := sha256.Sum256([]byte(now + in.Class + in.Symptom))
	in.ID = "INC-" + strings.ToUpper(hex.EncodeToString(digest[:]))[:10]
	in.Symptom, in.Expected, in.SuspectedCause, in.Recovery = redact(in.Symptom), redact(in.Expected), redact(in.SuspectedCause), redact(in.Recovery)
	all, _ := s.Incidents()
	for _, prior := range all {
		if prior.Class == in.Class && prior.Symptom == in.Symptom {
			in.RecurrenceCount = prior.RecurrenceCount + 1
			in.SimilarIncidentIDs = append(in.SimilarIncidentIDs, prior.ID)
		}
	}
	if in.RecurrenceCount == 0 {
		in.RecurrenceCount = 1
	}
	if err := appendUniqueJSONL(s.incidentPath(), in); err != nil {
		return Incident{}, err
	}
	for _, jobID := range in.JobIDs {
		if _, err := s.AppendJobEvent(jobID, "INCIDENT_OPENED", map[string]any{"incident_id": in.ID, "class": in.Class, "symptom": in.Symptom}); err != nil {
			return in, err
		}
	}
	return in, nil
}

func (s *Store) Incidents() ([]Incident, error) {
	var all []Incident
	if err := readJSONL(s.incidentPath(), &all); err != nil {
		return nil, err
	}
	// The incident log is append-only. Project repeated records for the same
	// incident ID to its newest state so a closeout does not appear twice.
	latest := make(map[string]Incident, len(all))
	order := make([]string, 0, len(all))
	for _, incident := range all {
		if _, seen := latest[incident.ID]; !seen {
			order = append(order, incident.ID)
		}
		latest[incident.ID] = incident
	}
	projected := make([]Incident, 0, len(order))
	for _, id := range order {
		projected = append(projected, latest[id])
	}
	return projected, nil
}

func (s *Store) CloseIncident(id, provenCause, fix, proof string) (Incident, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(provenCause) == "" || strings.TrimSpace(fix) == "" || strings.TrimSpace(proof) == "" {
		return Incident{}, errors.New("incident close requires proven cause, fix and regression proof")
	}
	all, err := s.Incidents()
	if err != nil {
		return Incident{}, err
	}
	for i := range all {
		if all[i].ID != id {
			continue
		}
		if all[i].Status == "RESOLVED" {
			return all[i], nil
		}
		all[i].Status, all[i].ProvenCause, all[i].Fix, all[i].RegressionProof = "RESOLVED", redact(provenCause), redact(fix), redact(proof)
		all[i].UpdatedAt, all[i].ClosedAt = time.Now().UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano)
		// Append a closeout record; the open record remains immutable evidence.
		if err := appendUniqueJSONL(s.incidentPath(), all[i]); err != nil {
			return Incident{}, err
		}
		for _, jobID := range all[i].JobIDs {
			if _, err := s.AppendJobEvent(jobID, "INCIDENT_RESOLVED", map[string]any{"incident_id": all[i].ID, "proof": all[i].RegressionProof}); err != nil {
				return all[i], err
			}
		}
		return all[i], nil
	}
	return Incident{}, fmt.Errorf("incident %q not found", id)
}

func (s *Store) AddLesson(lesson Lesson) (Lesson, error) {
	if strings.TrimSpace(lesson.Statement) == "" || strings.TrimSpace(lesson.Confidence) == "" || len(lesson.Evidence) == 0 {
		return Lesson{}, errors.New("lesson requires statement, confidence and evidence")
	}
	lesson.SchemaVersion, lesson.Status, lesson.CreatedAt = SchemaVersion, "candidate", time.Now().UTC().Format(time.RFC3339Nano)
	lesson.ID = "lesson-" + time.Now().UTC().Format("20060102-150405.000000000")
	lesson.Statement = redact(lesson.Statement)
	for i := range lesson.Evidence {
		lesson.Evidence[i] = redact(lesson.Evidence[i])
	}
	return lesson, appendUniqueJSONL(s.lessonPath(), lesson)
}

func (s *Store) LessonsFor(jobID string) ([]Lesson, error) {
	var all []Lesson
	if err := readJSONL(s.lessonPath(), &all); err != nil {
		return nil, err
	}
	var out []Lesson
	for _, lesson := range all {
		if contains(lesson.JobIDs, jobID) {
			out = append(out, lesson)
		}
	}
	return out, nil
}

func (s *Store) RegenerateAllNotes() error {
	records, err := s.Rebuild()
	if err != nil {
		return err
	}
	for _, record := range records {
		if _, err := s.RegenerateNote(record.JobID); err != nil {
			return err
		}
	}
	return nil
}

type Bundle struct {
	SchemaVersion      int               `json:"schema_version"`
	GeneratedAt        string            `json:"generated_at"`
	Gary               map[string]any    `json:"gary"`
	Job                HistoryRecord     `json:"job"`
	Packet             string            `json:"packet,omitempty"`
	PacketSHA256       string            `json:"packet_sha256,omitempty"`
	PacketTruncated    bool              `json:"packet_truncated,omitempty"`
	Events             []Event           `json:"events"`
	Workers            []Worker          `json:"workers,omitempty"`
	Acceptance         []Acceptance      `json:"acceptance,omitempty"`
	AcceptanceEvidence []map[string]any  `json:"acceptance_evidence,omitempty"`
	Incidents          []Incident        `json:"incidents,omitempty"`
	History            []HistoryRecord   `json:"recent_history,omitempty"`
	Configuration      map[string]string `json:"configuration"`
	NextAction         string            `json:"next_action"`
}

func (s *Store) DiagnosticBundle(jobID string, recent int) (Bundle, error) {
	record, err := s.Record(jobID)
	if err != nil {
		return Bundle{}, err
	}
	record.Title, record.Repo, record.PacketPath = redact(record.Title), redact(record.Repo), redact(record.PacketPath)
	_, state, err := s.findState(jobID)
	if err != nil {
		return Bundle{}, err
	}
	events, err := readEvents(record.JournalPath)
	if err != nil {
		return Bundle{}, err
	}
	for i := range events {
		events[i] = redactEvent(events[i])
	}
	incidents, _ := s.Incidents()
	var related []Incident
	for _, incident := range incidents {
		if contains(incident.JobIDs, jobID) {
			related = append(related, incident)
		}
	}
	history, _ := s.Records()
	if recent < 1 {
		recent = 10
	}
	if len(history) > recent {
		history = history[:recent]
	}
	for i := range history {
		history[i] = compactHistoryRecord(history[i])
	}
	packet, packetSHA, packetTruncated := "", "", false
	if record.PacketPath != "" {
		if data, readErr := os.ReadFile(record.PacketPath); readErr == nil {
			digest := sha256.Sum256(data)
			packetSHA = hex.EncodeToString(digest[:])
			packet = redact(string(data))
			packet, packetTruncated = limitText(packet, 64*1024)
		}
	}
	workers := redactWorkers(state.Workers, state.Worker)
	acceptanceEvidence := make([]map[string]any, 0)
	for _, worker := range workers {
		if worker.AcceptancePath == "" {
			continue
		}
		if data, readErr := os.ReadFile(worker.AcceptancePath); readErr == nil {
			var evidence map[string]any
			if json.Unmarshal(data, &evidence) == nil {
				acceptanceEvidence = append(acceptanceEvidence, redactMap(evidence))
			}
		}
	}
	generatedAt := record.UpdatedAt
	if generatedAt == "" {
		generatedAt = record.StartedAt
	}
	return Bundle{SchemaVersion: SchemaVersion, GeneratedAt: generatedAt, Gary: map[string]any{"version": "local", "commit": gitHead(), "go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH}, Job: record, Packet: packet, PacketSHA256: packetSHA, PacketTruncated: packetTruncated, Events: events, Workers: workers, Acceptance: record.Acceptance, AcceptanceEvidence: acceptanceEvidence, Incidents: related, History: history, Configuration: redactedEnvironment(), NextAction: record.NextAction}, nil
}

func limitText(value string, max int) (string, bool) {
	if len(value) <= max {
		return value, false
	}
	return value[:max] + "\n[TRUNCATED BY GARY BLACK BOX]\n", true
}

func compactHistoryRecord(record HistoryRecord) HistoryRecord {
	record.Task, _ = limitText(record.Task, 512)
	record.AcceptanceCommands = nil
	record.Workers = nil
	record.Artifacts = nil
	record.RelatedJobs = nil
	record.Lessons = nil
	record.Decisions, _ = limitStrings(record.Decisions, 5, 256)
	return record
}

func limitStrings(values []string, count, size int) ([]string, bool) {
	if len(values) > count {
		values = values[:count]
	}
	truncated := false
	out := make([]string, len(values))
	for i, value := range values {
		var itemTruncated bool
		out[i], itemTruncated = limitText(value, size)
		truncated = truncated || itemTruncated
	}
	return out, truncated
}

func derivedAt(records []HistoryRecord) string {
	latest := ""
	for _, record := range records {
		candidate := record.UpdatedAt
		if candidate == "" {
			candidate = record.FinishedAt
		}
		if candidate == "" {
			candidate = record.StartedAt
		}
		if candidate > latest {
			latest = candidate
		}
	}
	return latest
}

func gitHead() string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = "."
	if output, err := cmd.Output(); err == nil {
		return strings.TrimSpace(string(output))
	}
	return "UNKNOWN"
}

func redactedEnvironment() map[string]string {
	out := map[string]string{}
	for _, key := range []string{"GARY_STATE_ROOT", "GARY_STATE_DIR", "GARY_YARDMASTER_MODEL", "GARY_MIMO_BASE_URL", "GARY_ENABLE_BATCH"} {
		if value := os.Getenv(key); value != "" {
			out[key] = redact(value)
		}
	}
	return out
}

func redactWorkers(workers []Worker, current *Worker) []Worker {
	if len(workers) == 0 && current != nil {
		workers = []Worker{*current}
	}
	out := make([]Worker, len(workers))
	for i, worker := range workers {
		out[i] = redactWorker(worker)
		out[i].ProcessAlive = processAlive(worker.PID)
	}
	return out
}
func redactWorker(worker Worker) Worker {
	worker.Worktree, worker.LogPath, worker.AcceptancePath = redactPath(worker.Worktree), redactPath(worker.LogPath), redactPath(worker.AcceptancePath)
	return worker
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	if runtime.GOOS == "windows" {
		output, err := exec.Command("tasklist", "/FI", "PID eq "+strconv.Itoa(pid), "/FO", "CSV", "/NH").CombinedOutput()
		return err == nil && strings.Contains(string(output), strconv.Itoa(pid))
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}
func redactPath(value string) string { return redact(value) }
func redactEvent(event Event) Event  { event.Data = redactMap(event.Data); return event }
func redactMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "password") || strings.Contains(lower, "api_key") || strings.Contains(lower, "credential") {
			out[key] = "[REDACTED]"
		} else {
			out[key] = redactValue(value)
		}
	}
	return out
}
func redactValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return redactMap(v)
	case []any:
		out := make([]any, len(v))
		for i := range v {
			out[i] = redactValue(v[i])
		}
		return out
	case string:
		return redact(v)
	default:
		return value
	}
}
func redact(value string) string {
	if strings.TrimSpace(value) == "" {
		return value
	}
	return sensitivePattern.ReplaceAllString(value, "$1=[REDACTED]")
}

// RedactSensitive is used at boundaries that may persist operator/model input
// in logs or diagnostic material. It preserves ordinary text and masks common
// credential forms.
func RedactSensitive(value string) string { return redact(value) }

var sensitivePattern = regexp.MustCompile(`(?i)(api[_-]?key|token|password|secret|authorization|bearer)(\s*[:=]\s*|\s+)[^\s,;]+`)

func appendUniqueJSONL(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	lock := path + ".lock"
	var file *os.File
	for attempt := 0; attempt < 2; attempt++ {
		file, err = os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = fmt.Fprintf(file, "%d\n", os.Getpid())
			break
		}
		if !os.IsExist(err) {
			return err
		}
		owner, readErr := os.ReadFile(lock)
		pid, parseErr := strconv.Atoi(strings.TrimSpace(string(owner)))
		if readErr == nil && parseErr == nil && processAlive(pid) {
			return fmt.Errorf("evidence writer busy: process %d holds the lock", pid)
		}
		if removeErr := os.Remove(lock); removeErr != nil && !os.IsNotExist(removeErr) {
			return removeErr
		}
	}
	if file == nil {
		return errors.New("could not acquire evidence writer lock")
	}
	_ = file.Close()
	defer os.Remove(lock)
	if existing, readErr := os.ReadFile(path); readErr == nil && bytes.Contains(existing, data) {
		return nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(data, '\n'))
	return err
}

func readJSONL(path string, target any) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	lines := bytes.Split(data, []byte("\n"))
	values := make([]json.RawMessage, 0, len(lines))
	for _, line := range lines {
		if len(bytes.TrimSpace(line)) > 0 {
			values = append(values, append([]byte(nil), line...))
		}
	}
	encoded, _ := json.Marshal(values)
	return json.Unmarshal(encoded, target)
}
func atomicWrite(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".blackbox-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func title(task, fallback string) string {
	first := strings.TrimSpace(strings.SplitN(task, "\n", 2)[0])
	if first == "" {
		return fallback
	}
	if len(first) > 140 {
		return first[:140]
	}
	return first
}
func valueOrUnknown(value string) string {
	if strings.TrimSpace(value) == "" {
		return "UNKNOWN"
	}
	return value
}
func nextAction(state stateFile, workers []Worker) string {
	switch state.Status {
	case "DONE":
		return "No further action recorded."
	case "FAILED":
		return "Inspect the journal and create an explicit repair or retry job."
	case "CANCELLED":
		return "Reconcile preserved work before deciding whether to retry."
	case "NEEDS_MATTHEW":
		if state.Blocker != nil {
			return state.Blocker.UnblockAction
		}
		return "Inspect the persisted human boundary."
	case "RUNNING":
		for _, worker := range workers {
			if worker.Status == "RUNNING" {
				return "Reconcile the existing worker; do not spawn a replacement."
			}
		}
		return "Reconcile worker evidence and continue the same durable job."
	default:
		return "Inspect durable state before acting."
	}
}
func isTerminalEvent(typ string) bool {
	return typ == "JOB_DONE" || typ == "JOB_FAILED" || typ == "JOB_CANCELLED"
}
func isMeaningfulEvent(typ string) bool { return typ != "JOB_CREATED" && typ != "PREFLIGHT_COMPLETED" }
func eventDataHint(data map[string]any) string {
	if data == nil {
		return ""
	}
	if value := stringValue(data, "reason"); value != "" {
		return ": " + value
	}
	if value := stringValue(data, "summary"); value != "" {
		return ": " + value
	}
	return ""
}
func stringValue(data map[string]any, key string) string {
	if data == nil {
		return ""
	}
	if value, ok := data[key].(string); ok {
		return redact(value)
	}
	return ""
}
func boolValue(data map[string]any, key string) bool {
	value, ok := data[key]
	if !ok {
		return false
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(v, "true")
	}
	return false
}
func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func appendUnique(values []string, target string) []string {
	if !contains(values, target) {
		return append(values, target)
	}
	return values
}
func uniqueSorted(values []string) []string {
	values = uniqueStrings(values)
	sort.Strings(values)
	return values
}
func uniqueStrings(values []string) []string {
	out := []string{}
	for _, value := range values {
		out = appendUnique(out, value)
	}
	return out
}

func redactStrings(values []string) []string {
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = redact(value)
	}
	return out
}

// ParseSince accepts the deliberately small CLI grammar: 7d, 12h, 30m, or an
// RFC3339 timestamp. It returns false for an invalid filter instead of
// silently broadening the query.
func ParseSince(value string, now time.Time) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	if len(value) > 1 {
		unit := value[len(value)-1]
		if unit == 'd' || unit == 'h' || unit == 'm' {
			n, err := strconv.Atoi(value[:len(value)-1])
			if err != nil || n < 0 {
				return time.Time{}, errors.New("since must be N[mdh] or RFC3339")
			}
			d := time.Minute
			if unit == 'h' {
				d = time.Hour
			}
			if unit == 'd' {
				d = 24 * time.Hour
			}
			return now.Add(-time.Duration(n) * d), nil
		}
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, errors.New("since must be N[mdh] or RFC3339")
	}
	return parsed, nil
}
