package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/matthewjameswatkins1978-cyber/gary/control"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/batch"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/config"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/gitops"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/jobs"
)

const (
	issueCommandTitle   = "GARY COMMAND"
	trustedGitHubUser   = "matthewjameswatkins1978-cyber"
	trustedGitHubUserID = 249632099
	trustedGaryRepo     = "matthewjameswatkins1978-cyber/gary"
)

var garyBatchJobID = regexp.MustCompile(`^gary-(?:run|closer|smoke|benchmark)-[a-z0-9][a-z0-9-]{0,61}$`)

// issueEvent is the minimal, verified subset of GitHub's issues event.
type issueEvent struct {
	Action     string `json:"action"`
	Repository struct {
		FullName string `json:"full_name"`
		ID       int64  `json:"id"`
	} `json:"repository"`
	Sender struct {
		Login string `json:"login"`
		ID    int64  `json:"id"`
	} `json:"sender"`
	Issue struct {
		Number      int    `json:"number"`
		Title       string `json:"title"`
		Body        string `json:"body"`
		PullRequest any    `json:"pull_request"`
		User        struct {
			Login string `json:"login"`
			ID    int64  `json:"id"`
		} `json:"user"`
	} `json:"issue"`
}

type issueCommand struct {
	Version    int       `json:"version"`
	Command    string    `json:"command"`
	Job        *issueJob `json:"job,omitempty"`
	BatchJobID string    `json:"batch_job_id,omitempty"`
	LogLimit   int       `json:"log_limit,omitempty"`
	MaxJobs    int       `json:"max_jobs,omitempty"`
}

// issueJob deliberately uses acceptance profiles, never commands supplied by
// an issue body. Profiles are mapped to fixed executable arguments below.
type issueJob struct {
	ID         string   `json:"id"`
	Repo       string   `json:"repo"`
	BaseBranch string   `json:"base_branch"`
	Task       string   `json:"task"`
	Acceptance []string `json:"acceptance"`
}

type issueResult struct {
	OK      bool   `json:"ok"`
	Close   bool   `json:"close"`
	Comment string `json:"comment"`
}

func cmdIssue() {
	if len(os.Args) < 4 || len(os.Args) > 5 {
		fmt.Fprintln(os.Stderr, "Usage: gary issue <validate|execute> <github-event.json> [result.json]")
		os.Exit(1)
	}
	eventPath := os.Args[3]
	event, command, err := loadIssueCommand(eventPath)
	if err != nil {
		if os.Args[2] == "execute" && len(os.Args) == 5 {
			_ = saveIssueResult(os.Args[4], issueFailure(err))
		}
		fmt.Fprintf(os.Stderr, "issue: %v\n", err)
		os.Exit(1)
	}
	if os.Args[2] == "validate" {
		fmt.Printf("GARY // ISSUE VALID\nIssue: %d\nCommand: %s\n", event.Issue.Number, command.Command)
		return
	}
	if os.Args[2] != "execute" {
		fmt.Fprintln(os.Stderr, "Usage: gary issue <validate|execute> <github-event.json> [result.json]")
		os.Exit(1)
	}

	result := executeIssueCommand(command)
	if len(os.Args) == 5 {
		if err := saveIssueResult(os.Args[4], result); err != nil {
			fmt.Fprintf(os.Stderr, "issue result: %v\n", err)
			os.Exit(1)
		}
	}
	if !result.OK {
		fmt.Fprintln(os.Stderr, result.Comment)
		os.Exit(1)
	}
	fmt.Println(result.Comment)
}

func loadIssueCommand(path string) (*issueEvent, *issueCommand, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read GitHub event: %w", err)
	}
	var event issueEvent
	// GitHub's event envelope evolves independently of GARY. Decode only the
	// fields we authenticate; strictness belongs to the command body below.
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, nil, fmt.Errorf("parse GitHub event: %w", err)
	}
	if err := validateIssueEvent(&event); err != nil {
		return nil, nil, err
	}
	command, err := parseIssueCommandBody(event.Issue.Body)
	if err != nil {
		return nil, nil, err
	}
	if err := validateIssueCommand(command); err != nil {
		return nil, nil, err
	}
	return &event, command, nil
}

func validateIssueEvent(event *issueEvent) error {
	if event.Action != "opened" {
		return errors.New("only newly opened issues are accepted")
	}
	if event.Repository.FullName != trustedGaryRepo || event.Repository.ID != 1344689742 {
		return errors.New("event is not from the trusted GARY repository")
	}
	if event.Sender.Login != trustedGitHubUser || event.Sender.ID != trustedGitHubUserID ||
		event.Issue.User.Login != trustedGitHubUser || event.Issue.User.ID != trustedGitHubUserID {
		return errors.New("issue author is not the trusted GARY operator")
	}
	if event.Issue.PullRequest != nil {
		return errors.New("pull requests are not GARY commands")
	}
	if event.Issue.Title != issueCommandTitle {
		return fmt.Errorf("title must be exactly %q", issueCommandTitle)
	}
	return nil
}

func parseIssueCommandBody(body string) (*issueCommand, error) {
	body = strings.TrimSpace(body)
	if strings.HasPrefix(body, "```json\n") && strings.HasSuffix(body, "\n```") {
		body = strings.TrimSuffix(strings.TrimPrefix(body, "```json\n"), "\n```")
	}
	if body == "" {
		return nil, errors.New("body must contain exactly one JSON command object")
	}
	decoder := json.NewDecoder(strings.NewReader(body))
	decoder.DisallowUnknownFields()
	var command issueCommand
	if err := decoder.Decode(&command); err != nil {
		return nil, fmt.Errorf("parse command JSON: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("command body must contain exactly one JSON object")
	}
	return &command, nil
}

func validateIssueCommand(command *issueCommand) error {
	if command.Version != 1 {
		return errors.New("version must be 1")
	}
	switch command.Command {
	case "status", "doctor", "list":
		if command.Job != nil || command.BatchJobID != "" || command.LogLimit != 0 {
			return errors.New("this command accepts no job or batch_job_id")
		}
		if command.MaxJobs < 0 || command.MaxJobs > 50 {
			return errors.New("max_jobs must be between 1 and 50 when set")
		}
	case "inspect", "logs", "cancel":
		if command.Job != nil || !garyBatchJobID.MatchString(command.BatchJobID) {
			return errors.New("command requires one exact GARY batch_job_id")
		}
		if command.MaxJobs != 0 || command.LogLimit < 0 || command.LogLimit > 50 {
			return errors.New("invalid list or log limit")
		}
		if (command.Command == "cancel") && protectedJob(command.BatchJobID) {
			return errors.New("JC-01 and JC-02 are protected from control-plane cancellation")
		}
	case "submit", "recover":
		if command.Job == nil {
			return errors.New("command requires a bounded job manifest")
		}
		if err := validateIssueJob(command.Job); err != nil {
			return err
		}
		if command.Command == "recover" {
			if !garyBatchJobID.MatchString(command.BatchJobID) {
				return errors.New("recover requires one exact GARY batch_job_id")
			}
			if protectedJob(command.BatchJobID) {
				return errors.New("JC-01 and JC-02 are protected from control-plane recovery")
			}
		} else if command.BatchJobID != "" {
			return errors.New("submit does not accept batch_job_id")
		}
		if command.LogLimit != 0 || command.MaxJobs != 0 {
			return errors.New("submit and recover accept no list or log limit")
		}
	default:
		return errors.New("command must be one of submit, list, inspect, logs, cancel, doctor, status, recover")
	}
	return nil
}

func protectedJob(id string) bool {
	return strings.Contains(id, "-jc-01-") || strings.Contains(id, "-jc-02-")
}

func validateIssueJob(job *issueJob) error {
	if job.ID == "jc-01" || job.ID == "jc-02" {
		return errors.New("JC-01 and JC-02 are protected from issue submission")
	}
	acceptance, err := issueAcceptance(job.Acceptance)
	if err != nil {
		return err
	}
	return validateInboxJob(&jobs.Job{ID: job.ID, Repo: job.Repo, BaseBranch: job.BaseBranch, Task: job.Task, Acceptance: acceptance})
}

func issueAcceptance(profiles []string) ([]string, error) {
	if len(profiles) == 0 || len(profiles) > 3 {
		return nil, errors.New("acceptance must contain one to three approved profiles")
	}
	allowed := map[string]string{
		"go_test_all":             "go test ./...",
		"go_test_all_count_1":     "go test ./... -count=1",
		"cargo_test":              "cargo test",
		"cargo_test_all_features": "cargo test --all-features",
		"npm_test":                "npm test",
		"python_pytest_q":           "python -m pytest -q",
		"python_install_dev_deploy": "python -m pip install -e .[dev,deploy]",
		"ruff_check_all":            "ruff check .",
		"python_build":              "python -m build",
	}
	commands := make([]string, 0, len(profiles))
	seen := make(map[string]bool)
	for _, profile := range profiles {
		command, ok := allowed[profile]
		if !ok {
			return nil, fmt.Errorf("acceptance profile %q is not approved", profile)
		}
		if seen[profile] {
			return nil, fmt.Errorf("acceptance profile %q is repeated", profile)
		}
		seen[profile] = true
		commands = append(commands, command)
	}
	return commands, nil
}

func executeIssueCommand(command *issueCommand) issueResult {
	cfg, err := issueConfig()
	if err != nil {
		return issueFailure(err)
	}
	switch command.Command {
	case "status":
		items, err := batch.ListJobs(cfg.GCPProject, cfg.GCPRegion, 20)
		if err != nil {
			return issueFailure(err)
		}
		return issueSuccess(formatJobList("GARY // STATUS", items))
	case "list":
		limit := command.MaxJobs
		if limit == 0 {
			limit = 20
		}
		items, err := batch.ListJobs(cfg.GCPProject, cfg.GCPRegion, limit)
		if err != nil {
			return issueFailure(err)
		}
		return issueSuccess(formatJobList("GARY // BATCH JOBS", items))
	case "inspect":
		status, err := batch.DescribeJob(cfg.GCPProject, cfg.GCPRegion, command.BatchJobID)
		if err != nil {
			return issueFailure(err)
		}
		return issueSuccess(fmt.Sprintf("GARY // INSPECT\nJob: %s\nState: %s\nUID: %s", command.BatchJobID, status.State(), status.UID))
	case "logs":
		status, err := batch.DescribeJob(cfg.GCPProject, cfg.GCPRegion, command.BatchJobID)
		if err != nil {
			return issueFailure(err)
		}
		limit := command.LogLimit
		if limit == 0 {
			limit = 20
		}
		logs, err := batch.RecentLogs(cfg.GCPProject, status.UID, limit)
		if err != nil {
			return issueFailure(err)
		}
		return issueSuccess("GARY // LOGS\nJob: " + command.BatchJobID + "\nState: " + status.State() + "\n\n" + truncateIssueText(logs, 10000))
	case "cancel":
		status, err := batch.DescribeJob(cfg.GCPProject, cfg.GCPRegion, command.BatchJobID)
		if err != nil {
			return issueFailure(err)
		}
		if batch.IsTerminal(status.State()) {
			return issueSuccess(fmt.Sprintf("GARY // CANCEL\nJob: %s\nAlready terminal: %s", command.BatchJobID, status.State()))
		}
		if err := batch.CancelJob(cfg.GCPProject, cfg.GCPRegion, command.BatchJobID); err != nil {
			return issueFailure(err)
		}
		return issueSuccess("GARY // CANCEL\nCancellation requested for " + command.BatchJobID)
	case "doctor":
		return runCloudDoctor(cfg)
	case "submit":
		if os.Getenv("GARY_GITHUB_TOKEN") == "" {
			return issueFailure(errors.New("submit is locked until the dedicated GARY GitHub App token is configured"))
		}
		job, _ := jobFromIssue(command.Job)
		result := RunJob(cfg, job, "issue")
		return issueSuccess(formatRunResult("GARY // SUBMIT", result))
	case "recover":
		if os.Getenv("GARY_GITHUB_TOKEN") == "" {
			return issueFailure(errors.New("recover is locked until the dedicated GARY GitHub App token is configured"))
		}
		job, _ := jobFromIssue(command.Job)
		result, err := recoverBatchRun(cfg, job, command.BatchJobID)
		if err != nil {
			return issueFailure(err)
		}
		return issueSuccess(formatRunResult("GARY // RECOVER", result))
	}
	return issueFailure(errors.New("unreachable issue command"))
}

func issueConfig() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if cfg.GCPProject == "" || cfg.GCPRegion == "" {
		return nil, errors.New("GARY_GCP_PROJECT and GARY_GCP_REGION are required")
	}
	return cfg, nil
}

func jobFromIssue(input *issueJob) (*jobs.Job, error) {
	acceptance, err := issueAcceptance(input.Acceptance)
	if err != nil {
		return nil, err
	}
	return &jobs.Job{ID: input.ID, Repo: input.Repo, BaseBranch: input.BaseBranch, Task: input.Task, Acceptance: acceptance}, nil
}

func recoverBatchRun(cfg *config.Config, job *jobs.Job, batchJobID string) (jobs.RunResult, error) {
	status, err := batch.DescribeJob(cfg.GCPProject, cfg.GCPRegion, batchJobID)
	if err != nil {
		return jobs.RunResult{}, err
	}
	if status.State() != "SUCCEEDED" {
		return jobs.RunResult{}, fmt.Errorf("recovery requires a SUCCEEDED Batch job, found %s", status.State())
	}
	patchData := extractFromBatchLogs(cfg.GCPProject, cfg.GCPRegion, batchJobID, "GARY_PATCH_B64")
	if patchData == "" {
		return jobs.RunResult{}, errors.New("no recoverable patch markers found in Cloud Logging")
	}
	workDir, err := os.MkdirTemp("", "gary-recover-*")
	if err != nil {
		return jobs.RunResult{}, fmt.Errorf("create recovery directory: %w", err)
	}
	defer os.RemoveAll(workDir)
	sum := sha256.Sum256([]byte(batchJobID))
	branch := fmt.Sprintf("gary/recovered-%s-%x", job.ID, sum[:4])
	if !runLocalAcceptance(workDir, job, branch, patchData, "recover") {
		return jobs.RunResult{}, errors.New("recovered patch did not pass local acceptance")
	}
	repoDir := filepath.Join(workDir, "repo")
	commit, _ := gitops.CommitSHA(repoDir)
	if err := gitops.Push(repoDir, "origin", branch); err != nil {
		return jobs.RunResult{}, fmt.Errorf("push recovered branch: %w", err)
	}
	return jobs.RunResult{JobID: job.ID, BatchJobID: batchJobID, Branch: branch, CommitSHA: commit, Accepted: true, PatchBytes: int64(len(patchData)), Attempts: 1, FinalStatus: "DONE"}, nil
}

func runCloudDoctor(cfg *config.Config) issueResult {
	items, err := batch.ListJobs(cfg.GCPProject, cfg.GCPRegion, 1)
	if err != nil {
		return issueFailure(fmt.Errorf("Batch API check: %w", err))
	}
	result := fmt.Sprintf("GARY // CLOUD DOCTOR\nProject: %s\nRegion: %s\nBatch API: PASS\nLogging access: not queried until an exact job is requested\nResult bucket access: reserved for exact recovery", cfg.GCPProject, cfg.GCPRegion)
	if len(items) == 0 {
		result += "\nRecent GARY jobs: none"
	} else {
		result += "\nRecent GARY job: " + shortJobName(items[0].Name) + " (" + items[0].State + ")"
	}
	return issueSuccess(result)
}

func formatJobList(title string, jobs []batch.JobSummary) string {
	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n")
	if len(jobs) == 0 {
		b.WriteString("No jobs found.")
		return b.String()
	}
	for _, job := range jobs {
		fmt.Fprintf(&b, "- %s — %s — %s\n", shortJobName(job.Name), job.State, job.CreateTime)
	}
	return strings.TrimSpace(b.String())
}

func shortJobName(name string) string {
	if slash := strings.LastIndex(name, "/"); slash >= 0 {
		return name[slash+1:]
	}
	return name
}

func formatRunResult(title string, result jobs.RunResult) string {
	return fmt.Sprintf("%s\nStatus: %s\nBatch job: %s\nBranch: %s\nCommit: %s\nAccepted: %t\nAttempts: %d", title, result.FinalStatus, result.BatchJobID, result.Branch, result.CommitSHA, result.Accepted, result.Attempts)
}

func issueSuccess(comment string) issueResult {
	return issueResult{OK: true, Close: true, Comment: truncateIssueText(comment, 11000)}
}
func issueFailure(err error) issueResult {
	class := control.ClassifyError(err, "")
	comment := fmt.Sprintf("GARY // COMMAND FAILED\nclass: %s\nerror: %s\naction: %s",
		class, err.Error(), control.SuggestNextAction(class))
	return issueResult{OK: false, Close: true, Comment: truncateIssueText(comment, 11000)}
}

func truncateIssueText(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max] + "\n… truncated"
}

func saveIssueResult(path string, result issueResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}
