package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/blackbox"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/config"
)

func blackBoxStore() *blackbox.Store {
	root := os.Getenv("GARY_STATE_DIR")
	if root == "" {
		root = config.StateRoot()
	}
	store, err := blackbox.New(root)
	if err != nil {
		fatalBlackBox(err)
	}
	return store
}

func fatalBlackBox(err error) { fmt.Fprintf(os.Stderr, "GARY BLACK BOX: %v\n", err); os.Exit(1) }

func cmdJobs() { cmdHistoryList(true) }

func cmdRecent() {
	args := append([]string{"--limit", "10"}, os.Args[2:]...)
	cmdHistoryArgs(args, true)
}

func cmdHistory() { cmdHistoryArgs(os.Args[2:], false) }

type historyOptions struct {
	repo, project, status, worker, since, search string
	limit                                        int
	json                                         bool
}

func parseHistoryOptions(args []string) (historyOptions, []string, error) {
	options := historyOptions{limit: 20}
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			positional = append(positional, arg)
			continue
		}
		name, value, has := strings.Cut(arg, "=")
		need := func() (string, error) {
			if has {
				return value, nil
			}
			if i+1 >= len(args) {
				return "", fmt.Errorf("%s requires a value", name)
			}
			i++
			return args[i], nil
		}
		switch name {
		case "--repo":
			v, e := need()
			options.repo = v
			if e != nil {
				return options, nil, e
			}
		case "--project":
			v, e := need()
			options.project = v
			if e != nil {
				return options, nil, e
			}
		case "--status":
			v, e := need()
			options.status = v
			if e != nil {
				return options, nil, e
			}
		case "--failed":
			options.status = "FAILED"
		case "--worker":
			v, e := need()
			options.worker = v
			if e != nil {
				return options, nil, e
			}
		case "--since":
			v, e := need()
			options.since = v
			if e != nil {
				return options, nil, e
			}
		case "--search":
			v, e := need()
			options.search = v
			if e != nil {
				return options, nil, e
			}
		case "--limit":
			v, e := need()
			if e != nil {
				return options, nil, e
			}
			n, parseErr := strconv.Atoi(v)
			if parseErr != nil || n < 1 {
				return options, nil, errors.New("--limit must be a positive integer")
			}
			options.limit = n
		case "--json":
			options.json = true
		default:
			return options, nil, fmt.Errorf("unknown option %q", arg)
		}
	}
	return options, positional, nil
}

func cmdHistoryArgs(args []string, jobsAlias bool) {
	if len(args) > 0 && args[0] == "rebuild" {
		if len(args) != 1 {
			fatalBlackBox(errors.New("usage: gary history rebuild"))
		}
		records, err := blackBoxStore().Rebuild()
		if err != nil {
			fatalBlackBox(err)
		}
		fmt.Printf("Rebuilt GARY history from raw job evidence: %d jobs\n", len(records))
		return
	}
	if len(args) > 0 && args[0] == "show" {
		if len(args) != 2 {
			fatalBlackBox(errors.New("usage: gary history show <job-id>"))
		}
		cmdHistoryShow(args[1])
		return
	}
	if len(args) > 0 && args[0] == "search" {
		if len(args) < 2 || strings.TrimSpace(strings.Join(args[1:], " ")) == "" {
			fatalBlackBox(errors.New("usage: gary history search <text>"))
		}
		args = append([]string{"--search", strings.Join(args[1:], " ")}, args[0:0]...)
	}
	if len(args) > 0 && args[0] == "context" {
		cmdHistoryContext(args[1:])
		return
	}
	options, positional, err := parseHistoryOptions(args)
	if err != nil {
		fatalBlackBox(err)
	}
	if len(positional) > 0 {
		fatalBlackBox(fmt.Errorf("unknown history argument %q", positional[0]))
	}
	if jobsAlias {
		options.limit = 1000
	}
	store := blackBoxStore()
	records, err := store.Records()
	if err != nil {
		fatalBlackBox(err)
	}
	since, err := blackbox.ParseSince(options.since, time.Now().UTC())
	if err != nil {
		fatalBlackBox(err)
	}
	filtered := filterRecords(records, options, since)
	if options.json {
		printJSON(filtered)
		return
	}
	if len(filtered) == 0 {
		fmt.Println("GARY HISTORY: no matching jobs")
		return
	}
	fmt.Println("GARY HISTORY")
	for _, record := range filtered {
		fmt.Printf("%s  %-16s %-10s %s\n", dateOrUnknown(record.StartedAt), record.Status, record.JobID, record.Title)
		fmt.Printf("  repo=%s next=%s\n", record.Repo, record.NextAction)
	}
}

func cmdHistoryList(jobsAlias bool) { cmdHistoryArgs(nil, jobsAlias) }

func filterRecords(records []blackbox.HistoryRecord, options historyOptions, since time.Time) []blackbox.HistoryRecord {
	var out []blackbox.HistoryRecord
	for _, record := range records {
		if options.repo != "" && !strings.EqualFold(record.Repo, options.repo) && !strings.Contains(strings.ToLower(record.Repo), strings.ToLower(options.repo)) {
			continue
		}
		if options.project != "" && !strings.EqualFold(record.Project, options.project) {
			continue
		}
		if options.status != "" && !strings.EqualFold(record.Status, options.status) {
			continue
		}
		if options.worker != "" {
			found := false
			for _, worker := range record.Workers {
				if strings.EqualFold(worker.Profile, options.worker) {
					found = true
				}
			}
			if !found {
				continue
			}
		}
		if !since.IsZero() {
			started, parseErr := time.Parse(time.RFC3339Nano, record.StartedAt)
			if parseErr != nil || started.Before(since) {
				continue
			}
		}
		if options.search != "" && !strings.Contains(strings.ToLower(recordText(record)), strings.ToLower(options.search)) {
			continue
		}
		out = append(out, record)
		if options.limit > 0 && len(out) >= options.limit {
			break
		}
	}
	return out
}

func recordText(record blackbox.HistoryRecord) string {
	data, _ := json.Marshal(record)
	return string(data)
}

func cmdHistoryShow(jobID string) {
	record, err := blackBoxStore().Record(jobID)
	if err != nil {
		fatalBlackBox(err)
	}
	fmt.Printf("GARY JOB %s\n\nTITLE: %s\nSTATUS: %s\nREPO: %s\nSTARTED: %s\nFINISHED: %s\nLAST PROVEN ACTION: %s\nNEXT ACTION: %s\nEVENTS: %d\nJOURNAL: %s\n", record.JobID, record.Title, record.Status, record.Repo, record.StartedAt, dateOrUnknown(record.FinishedAt), valueOrUnknown(record.LastAction), record.NextAction, record.EventCount, record.JournalPath)
	if len(record.FailureCategories) > 0 {
		fmt.Printf("FAILURES: %s\n", strings.Join(record.FailureCategories, ", "))
	}
	if len(record.Acceptance) > 0 {
		passed := 0
		for _, result := range record.Acceptance {
			if result.Passed {
				passed++
			}
		}
		fmt.Printf("ACCEPTANCE: %d/%d recorded attempts passed\n", passed, len(record.Acceptance))
	}
	for _, worker := range record.Workers {
		fmt.Printf("WORKER: %s profile=%s status=%s branch=%s commit=%s\n", worker.ID, worker.Profile, worker.Status, valueOrUnknown(worker.Branch), valueOrUnknown(worker.CommitSHA))
	}
	for _, decision := range record.Decisions {
		fmt.Printf("DECISION: %s\n", decision)
	}
	for _, relation := range record.RelatedJobs {
		fmt.Printf("RELATED: %s -> %s (%s; evidence=%s)\n", relation.FromJobID, relation.ToJobID, relation.Kind, relation.Evidence)
	}
}

func cmdHistoryContext(args []string) {
	options, positional, err := parseHistoryOptions(args)
	if err != nil {
		fatalBlackBox(err)
	}
	if len(positional) > 0 {
		fatalBlackBox(errors.New("usage: gary history context [--repo <repo>] [--limit N]"))
	}
	store := blackBoxStore()
	records, err := store.Records()
	if err != nil {
		fatalBlackBox(err)
	}
	filtered := filterRecords(records, options, time.Time{})
	if options.repo == "" && options.project == "" {
		filtered = filtered[:min(len(filtered), options.limit)]
	}
	printJSON(map[string]any{"schema_version": blackbox.SchemaVersion, "repo": options.repo, "records": filtered, "principle": "History is derived from durable job evidence; it is not model recollection."})
}

func cmdBlackBoxDoctor(jobID string) {
	bundle, err := blackBoxStore().DiagnosticBundle(jobID, 10)
	if err != nil {
		fatalBlackBox(err)
	}
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		fatalBlackBox(err)
	}
	root := blackBoxStore().Root
	dir := filepath.Join(root, "blackbox", "bundles")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		fatalBlackBox(err)
	}
	path := filepath.Join(dir, safeFrontName(jobID)+".json")
	if err := writeAtomicBlackBox(path, data); err != nil {
		fatalBlackBox(err)
	}
	fmt.Printf("GARY DIAGNOSTIC BUNDLE\nPath: %s\nBytes: %d\nSecrets: redacted\n\n", path, len(data))
	fmt.Println(string(data))
}

func cmdShow(jobID string) { cmdHistoryShow(jobID) }

func cmdNote() {
	if len(os.Args) < 4 {
		fatalBlackBox(errors.New("usage: gary note <job-id> <text>"))
	}
	jobID, note := os.Args[2], strings.TrimSpace(strings.Join(os.Args[3:], " "))
	if note == "" {
		fatalBlackBox(errors.New("note text is required"))
	}
	store := blackBoxStore()
	if _, err := store.AppendJobEvent(jobID, "OPERATOR_NOTE", map[string]any{"text": note}); err != nil {
		fatalBlackBox(err)
	}
	path, err := store.RegenerateNote(jobID)
	if err != nil {
		fatalBlackBox(err)
	}
	_, _ = store.Rebuild()
	fmt.Printf("Recorded operator note for %s\nNote: %s\n", jobID, path)
}

func cmdIssues() {
	incidents, err := blackBoxStore().Incidents()
	if err != nil {
		fatalBlackBox(err)
	}
	openOnly := false
	for _, arg := range os.Args[2:] {
		if arg == "--open" {
			openOnly = true
		} else {
			fatalBlackBox(fmt.Errorf("unknown option %q", arg))
		}
	}
	shown := 0
	fmt.Println("GARY INCIDENTS")
	for _, incident := range incidents {
		if openOnly && incident.Status == "RESOLVED" {
			continue
		}
		shown++
		fmt.Printf("%s %-9s %-12s recurrence=%d\n  symptom: %s\n  expected: %s\n", incident.ID, incident.Status, incident.Class, incident.RecurrenceCount, incident.Symptom, incident.Expected)
		if incident.SuspectedCause != "" {
			fmt.Printf("  suspected: %s\n", incident.SuspectedCause)
		}
		if incident.ProvenCause != "" {
			fmt.Printf("  proven: %s\n", incident.ProvenCause)
		}
	}
	if shown == 0 {
		fmt.Println("No incidents recorded.")
	}
}

func cmdIncident() {
	args := os.Args[2:]
	if len(args) == 0 {
		fatalBlackBox(errors.New("usage: gary incident open|close ..."))
	}
	switch args[0] {
	case "open":
		var in blackbox.Incident
		var jobs string
		for i := 1; i < len(args); i++ {
			name, value, next, err := optionValue(args, i)
			if err != nil {
				fatalBlackBox(err)
			}
			if next {
				i++
			}
			switch name {
			case "--class":
				in.Class = value
			case "--symptom":
				in.Symptom = value
			case "--expected":
				in.Expected = value
			case "--suspected":
				in.SuspectedCause = value
			case "--recovery":
				in.Recovery = value
			case "--job":
				jobs = value
			default:
				fatalBlackBox(fmt.Errorf("unknown option %q", name))
			}
		}
		if jobs != "" {
			in.JobIDs = strings.Split(jobs, ",")
		}
		incident, err := blackBoxStore().OpenIncident(in)
		if err != nil {
			fatalBlackBox(err)
		}
		fmt.Printf("Opened %s recurrence=%d\n", incident.ID, incident.RecurrenceCount)
	case "close":
		if len(args) < 2 {
			fatalBlackBox(errors.New("usage: gary incident close <id> --proven <cause> --fix <fix> --proof <test-or-live-proof>"))
		}
		id := args[1]
		var proven, fix, proof string
		for i := 2; i < len(args); i++ {
			name, value, next, err := optionValue(args, i)
			if err != nil {
				fatalBlackBox(err)
			}
			if next {
				i++
			}
			switch name {
			case "--proven":
				proven = value
			case "--fix":
				fix = value
			case "--proof":
				proof = value
			default:
				fatalBlackBox(fmt.Errorf("unknown option %q", name))
			}
		}
		incident, err := blackBoxStore().CloseIncident(id, proven, fix, proof)
		if err != nil {
			fatalBlackBox(err)
		}
		fmt.Printf("Resolved %s with regression proof: %s\n", incident.ID, incident.RegressionProof)
	default:
		fatalBlackBox(fmt.Errorf("unknown incident action %q", args[0]))
	}
}

func cmdRelate() {
	if len(os.Args) < 5 {
		fatalBlackBox(errors.New("usage: gary relate <from-job> <to-job> <kind> --evidence <reference> [--reason text]"))
	}
	from, to, kind := os.Args[2], os.Args[3], os.Args[4]
	reason, evidence := "", ""
	for i := 5; i < len(os.Args); i++ {
		name, value, next, err := optionValue(os.Args[2:], i-2)
		if err != nil {
			fatalBlackBox(err)
		}
		if next {
			i++
		}
		switch name {
		case "--reason":
			reason = value
		case "--evidence":
			evidence = value
		default:
			fatalBlackBox(fmt.Errorf("unknown option %q", name))
		}
	}
	relation, err := blackBoxStore().AddRelationship(from, to, kind, reason, evidence)
	if err != nil {
		fatalBlackBox(err)
	}
	fmt.Printf("Recorded relation %s -> %s (%s)\n", relation.FromJobID, relation.ToJobID, relation.Kind)
}

func cmdLesson() {
	if len(os.Args) < 4 {
		fatalBlackBox(errors.New("usage: gary lesson <job-id> <statement> --confidence <level> --evidence <ref> --destination <code_fix|regression_test|operational_rule>"))
	}
	lesson := blackbox.Lesson{JobIDs: []string{os.Args[2]}, Evidence: []string{}}
	lesson.Statement = os.Args[3]
	for i := 4; i < len(os.Args); i++ {
		name, value, next, err := optionValue(os.Args[2:], i-2)
		if err != nil {
			fatalBlackBox(err)
		}
		if next {
			i++
		}
		switch name {
		case "--confidence":
			lesson.Confidence = value
		case "--evidence":
			lesson.Evidence = append(lesson.Evidence, value)
		case "--destination":
			lesson.Destination = value
		default:
			fatalBlackBox(fmt.Errorf("unknown option %q", name))
		}
	}
	if lesson.Destination == "" {
		lesson.Destination = "regression_test"
	}
	saved, err := blackBoxStore().AddLesson(lesson)
	if err != nil {
		fatalBlackBox(err)
	}
	fmt.Printf("Recorded lesson candidate %s (%s)\n", saved.ID, saved.Destination)
}

func cmdReview(jobID string) {
	record, err := blackBoxStore().Record(jobID)
	if err != nil {
		fatalBlackBox(err)
	}
	incidents, _ := blackBoxStore().Incidents()
	fmt.Println("EXPECTED")
	fmt.Printf("Goal: %s\nAcceptance commands: evidence declared in the immutable job packet/state\n\nOBSERVED\nStatus: %s\nLast proven action: %s\nWorkers: %d\n\nDIVERGENCES\n", record.Title, record.Status, valueOrUnknown(record.LastAction), len(record.Workers))
	if len(record.FailureCategories) == 0 && record.Status != "RUNNING" {
		fmt.Println("None recorded.")
	} else {
		for _, value := range record.FailureCategories {
			fmt.Printf("- %s\n", value)
		}
		if record.Status == "RUNNING" {
			fmt.Println("- Job has no terminal outcome yet.")
		}
	}
	fmt.Println("\nRELATED INCIDENTS")
	related := 0
	for _, incident := range incidents {
		if containsString(incident.JobIDs, jobID) {
			related++
			fmt.Printf("- %s %s: %s\n", incident.ID, incident.Status, incident.Symptom)
		}
	}
	if related == 0 {
		fmt.Println("None recorded.")
	}
	fmt.Println("\nUNPROVEN ASSUMPTIONS")
	if len(record.Acceptance) == 0 {
		fmt.Println("- No acceptance result has been recorded.")
	}
	if record.Status == "RUNNING" {
		fmt.Println("- Worker liveness requires a live reconciliation check; the journal alone does not prove it.")
	}
	if len(record.ResultCommits) == 0 {
		fmt.Println("- No result commit is recorded.")
	}
	fmt.Printf("\nCURRENT STATE\nStatus: %s\nNext action: %s\n\nSAFE NEXT ACTIONS\n- %s\n\nEXECUTIVE QUESTIONS\n", record.Status, record.NextAction, record.NextAction)
	if record.Status == "NEEDS_MATTHEW" {
		fmt.Println("- A persisted human boundary is awaiting Matthew's answer.")
	} else {
		fmt.Println("None recorded; ordinary diagnosis remains within GARY/Lucy's scope.")
	}
}

func cmdHealth() {
	records, err := blackBoxStore().Records()
	if err != nil {
		fatalBlackBox(err)
	}
	incidents, _ := blackBoxStore().Incidents()
	completed, failed, needs, recovered := 0, 0, 0, 0
	failureCounts := map[string]int{}
	for _, record := range records {
		switch record.Status {
		case "DONE":
			completed++
		case "FAILED":
			failed++
		case "NEEDS_MATTHEW":
			needs++
		}
		for _, tag := range record.Tags {
			if tag == "recovered-after-restart" {
				recovered++
			}
		}
		for _, category := range record.FailureCategories {
			failureCounts[category]++
		}
	}
	open, repeated, unverifiedFixes, knownRegressions := 0, 0, 0, 0
	priorityClass := ""
	priorityRecurrence := 0
	for _, incident := range incidents {
		if incident.Status != "RESOLVED" {
			open++
			if incident.Fix != "" && incident.RegressionProof == "" {
				unverifiedFixes++
			}
			if incident.RecurrenceCount > priorityRecurrence {
				priorityClass, priorityRecurrence = incident.Class, incident.RecurrenceCount
			}
		}
		if incident.RecurrenceCount > 1 {
			repeated++
			if incident.Status != "RESOLVED" {
				knownRegressions++
			}
		}
	}
	fmt.Printf("GARY HEALTH\n\nJobs completed: %d\nJobs failed: %d\nJobs needing Matthew: %d\nRecovered after restart: %d\nOpen incidents: %d\nRepeated incidents: %d\nUnverified fixes: %d\nKnown regressions: %d\n", completed, failed, needs, recovered, open, repeated, unverifiedFixes, knownRegressions)
	if len(records) < 3 {
		fmt.Println("Recent deterioration: INSUFFICIENT EVIDENCE (fewer than 3 recorded jobs)")
	} else {
		fmt.Println("Recent deterioration: UNKNOWN (no comparable time-series baseline recorded)")
	}
	if len(failureCounts) == 0 {
		fmt.Println("Most common failure: UNKNOWN")
	} else {
		best, count := "", 0
		for category, n := range failureCounts {
			if n > count {
				best, count = category, n
			}
		}
		fmt.Printf("Most common failure: %s (%d)\n", best, count)
	}
	if priorityClass != "" {
		fmt.Printf("Suggested engineering priority: inspect open %s (recurrence=%d)\n", priorityClass, priorityRecurrence)
	} else {
		fmt.Println("Suggested engineering priority: INSUFFICIENT EVIDENCE")
	}
	fmt.Println("Unverified fixes: use `gary incidents` and require regression proof before closeout.")
}

func optionValue(args []string, index int) (string, string, bool, error) {
	arg := args[index]
	if !strings.HasPrefix(arg, "--") {
		return "", "", false, fmt.Errorf("expected option, got %q", arg)
	}
	name, value, has := strings.Cut(arg, "=")
	if has {
		return name, value, false, nil
	}
	if index+1 >= len(args) {
		return "", "", false, fmt.Errorf("%s requires a value", name)
	}
	return name, args[index+1], true, nil
}
func printJSON(value any) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fatalBlackBox(err)
	}
	fmt.Println(string(data))
}
func writeAtomicBlackBox(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".bundle-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0o600); err != nil {
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
func dateOrUnknown(value string) string {
	if value == "" {
		return "UNKNOWN"
	}
	return value
}
func valueOrUnknown(value string) string {
	if strings.TrimSpace(value) == "" {
		return "UNKNOWN"
	}
	return value
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
