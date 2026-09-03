package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	BoardSchema      = "gary.project-board.v1"
	BoardInProcess   = "IN_PROCESS"
	BoardCompleted   = "COMPLETED"
	BoardCancelled   = "CANCELLED"
	boardLockTimeout = 5 * time.Second
)

// BoardEntry is the small, human-readable project lifecycle record. It is a
// projection of Yardmaster-owned lifecycle events, not a worker/job status
// feed. In particular, failures and pauses do not create extra board states.
type BoardEntry struct {
	ProjectID            string `json:"project_id"`
	ProjectName          string `json:"project_name,omitempty"`
	Status               string `json:"status"`
	StartedAt            string `json:"started_at"`
	CompletedAt          string `json:"completed_at,omitempty"`
	CancelledAt          string `json:"cancelled_at,omitempty"`
	MasterPacketDigest   string `json:"master_packet_digest"`
	ProjectManualVersion int    `json:"project_manual_version"`
	LastEvent            string `json:"last_event,omitempty"`
}

// ProjectBoard is the complete durable list of projects known to GARY.
type ProjectBoard struct {
	Schema    string       `json:"schema"`
	UpdatedAt string       `json:"updated_at"`
	Projects  []BoardEntry `json:"projects"`
}

// BoardPolicy is part of every newly generated project manual. It makes the
// board lifecycle explicit without turning the board into a second planner.
type BoardPolicy struct {
	InProcessStatus      string `json:"in_process_status"`
	CompletionStatus     string `json:"completion_status"`
	CancellationStatus   string `json:"cancellation_status,omitempty"`
	CompletionAuthority  string `json:"completion_authority"`
	CompletionCondition  string `json:"completion_condition"`
	CancellationCondition string `json:"cancellation_condition,omitempty"`
}

func DefaultBoardPolicy() BoardPolicy {
	return BoardPolicy{
		InProcessStatus: BoardInProcess, CompletionStatus: BoardCompleted, CancellationStatus: BoardCancelled,
		CompletionAuthority: "Yardmaster",
		CompletionCondition: "Set COMPLETED only after the Yardmaster accepts the finished project.",
		CancellationCondition: "Set CANCELLED only after an explicit authorised project cancellation.",
	}
}

func (p BoardPolicy) isZero() bool {
	return p.InProcessStatus == "" && p.CompletionStatus == "" && p.CancellationStatus == "" && p.CompletionAuthority == "" && p.CompletionCondition == "" && p.CancellationCondition == ""
}

func (p BoardPolicy) valid() bool {
	if p.InProcessStatus != BoardInProcess || p.CompletionStatus != BoardCompleted || p.CompletionAuthority != "Yardmaster" || p.CompletionCondition == "" {
		return false
	}
	if p.CancellationStatus != "" && p.CancellationStatus != BoardCancelled {
		return false
	}
	return true
}

// NormalizeBoardPolicy makes the lifecycle invariant owned by GARY rather
// than by the model's formatting. A model may omit the field or return a
// partial variant, but it cannot redefine who completes a project or what
// the two durable board states mean.
func NormalizeBoardPolicy(p BoardPolicy) BoardPolicy {
	if !p.valid() {
		return DefaultBoardPolicy()
	}
	if p.CancellationStatus == "" {
		p.CancellationStatus = BoardCancelled
	}
	if p.CancellationCondition == "" {
		p.CancellationCondition = "Set CANCELLED only after an explicit authorised project cancellation."
	}
	return p
}

func (s Store) boardPath() string { return filepath.Join(s.root, "board", "projects.json") }

func (s Store) LoadBoard() (ProjectBoard, error) {
	var board ProjectBoard
	if err := readJSON(s.boardPath(), &board); errors.Is(err, os.ErrNotExist) {
		return ProjectBoard{Schema: BoardSchema, Projects: []BoardEntry{}}, nil
	} else if err != nil {
		return ProjectBoard{}, err
	}
	if board.Schema != BoardSchema {
		return ProjectBoard{}, fmt.Errorf("invalid project board schema %q", board.Schema)
	}
	sort.Slice(board.Projects, func(i, j int) bool { return board.Projects[i].ProjectID < board.Projects[j].ProjectID })
	return board, nil
}

func (s Store) MarkProjectInProcess(p Project) error {
	return s.updateBoard(func(board *ProjectBoard, now string) error {
		for i := range board.Projects {
			if board.Projects[i].ProjectID != p.ProjectID {
				continue
			}
			// A completed board entry is monotonic and must not be resurrected by
			// a stale create/retry path.
			if board.Projects[i].Status == BoardCompleted || board.Projects[i].Status == BoardCancelled {
				return nil
			}
			board.Projects[i].ProjectName = p.ProjectName
			board.Projects[i].MasterPacketDigest = p.MasterPacketDigest
			board.Projects[i].ProjectManualVersion = p.ProjectManualVersion
			board.Projects[i].LastEvent = "project started"
			return nil
		}
		board.Projects = append(board.Projects, BoardEntry{
			ProjectID: p.ProjectID, ProjectName: p.ProjectName, Status: BoardInProcess,
			StartedAt: now, MasterPacketDigest: p.MasterPacketDigest,
			ProjectManualVersion: p.ProjectManualVersion, LastEvent: "project started",
		})
		return nil
	})
}

// MarkProjectCompleted is intentionally callable only by the project
// acceptance path. It refuses to write COMPLETED for a merely paused/failed
// or worker-accepted project.
func (s Store) MarkProjectCompleted(p Project) error {
	if p.State != "ACCEPTED" {
		return fmt.Errorf("project board completion requires ACCEPTED project state, got %s", p.State)
	}
	return s.updateBoard(func(board *ProjectBoard, now string) error {
		for i := range board.Projects {
			if board.Projects[i].ProjectID != p.ProjectID {
				continue
			}
			if board.Projects[i].Status == BoardCompleted {
				return nil
			}
			board.Projects[i].Status = BoardCompleted
			board.Projects[i].CompletedAt = now
			board.Projects[i].MasterPacketDigest = p.MasterPacketDigest
			board.Projects[i].ProjectManualVersion = p.ProjectManualVersion
			board.Projects[i].LastEvent = "Yardmaster accepted project"
			return nil
		}
		return fmt.Errorf("project %q is missing from board", p.ProjectID)
	})
}

// MarkProjectCancelled projects an explicit non-successful closure.
func (s Store) MarkProjectCancelled(p Project) error {
	if p.State != LifecycleCancelled {
		return fmt.Errorf("project board cancellation requires CANCELLED project state, got %s", p.State)
	}
	return s.updateBoard(func(board *ProjectBoard, now string) error {
		for i := range board.Projects {
			if board.Projects[i].ProjectID != p.ProjectID {
				continue
			}
			if board.Projects[i].Status == BoardCompleted || board.Projects[i].Status == BoardCancelled {
				return nil
			}
			board.Projects[i].Status = BoardCancelled
			board.Projects[i].CancelledAt = now
			board.Projects[i].MasterPacketDigest = p.MasterPacketDigest
			board.Projects[i].ProjectManualVersion = p.ProjectManualVersion
			board.Projects[i].LastEvent = "project explicitly cancelled"
			return nil
		}
		return fmt.Errorf("project %q is missing from board", p.ProjectID)
	})
}

func (s Store) updateBoard(update func(*ProjectBoard, string) error) error {
	path := s.boardPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return withProjectFileLock(path+".lock", boardLockTimeout, func() error {
		board, err := s.LoadBoard()
		if err != nil {
			return err
		}
		if board.Schema == "" {
			board.Schema = BoardSchema
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		if err := update(&board, now); err != nil {
			return err
		}
		board.Schema = BoardSchema
		board.UpdatedAt = now
		sort.Slice(board.Projects, func(i, j int) bool { return board.Projects[i].ProjectID < board.Projects[j].ProjectID })
		return writeJSON(path, board)
	})
}

func withProjectFileLock(path string, timeout time.Duration, fn func() error) error {
	deadline := time.Now().Add(timeout)
	for {
		err := os.Mkdir(path, 0o755)
		if err == nil {
			defer os.Remove(path)
			return fn()
		}
		if !errors.Is(err, os.ErrExist) || time.Now().After(deadline) {
			return fmt.Errorf("project state busy: %w", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
