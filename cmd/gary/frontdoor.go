package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/config"
)

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, "\n") }
func (s *stringList) Set(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("value cannot be empty")
	}
	*s = append(*s, value)
	return nil
}

type doOptions struct {
	packetFile    string
	repo          string
	base          string
	stateDir      string
	id            string
	workerProfile string
	acceptance    stringList
	job           string
}

// parseDoArgs deliberately does not use flag.FlagSet. Go's flag parser stops
// option parsing at the first positional argument, which made an explicit
// --accept silently become part of the job packet.
func parseDoArgs(args []string) (doOptions, error) {
	var options doOptions
	var positional []string
	optionsEnded := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if optionsEnded {
			positional = append(positional, arg)
			continue
		}
		if arg == "--" {
			optionsEnded = true
			continue
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positional = append(positional, arg)
			continue
		}
		name, value, hasValue := strings.Cut(arg, "=")
		needsValue := func() (string, error) {
			if hasValue {
				if strings.TrimSpace(value) == "" {
					return "", fmt.Errorf("%s requires a non-empty value", name)
				}
				return value, nil
			}
			if i+1 >= len(args) {
				return "", fmt.Errorf("%s requires a value", name)
			}
			i++
			if strings.TrimSpace(args[i]) == "" {
				return "", fmt.Errorf("%s requires a non-empty value", name)
			}
			return args[i], nil
		}
		switch name {
		case "--file":
			v, err := needsValue()
			if err != nil {
				return options, err
			}
			options.packetFile = v
		case "--repo":
			v, err := needsValue()
			if err != nil {
				return options, err
			}
			options.repo = v
		case "--base":
			v, err := needsValue()
			if err != nil {
				return options, err
			}
			options.base = v
		case "--state-dir":
			v, err := needsValue()
			if err != nil {
				return options, err
			}
			options.stateDir = v
		case "--id":
			v, err := needsValue()
			if err != nil {
				return options, err
			}
			options.id = v
		case "--worker-profile":
			v, err := needsValue()
			if err != nil {
				return options, err
			}
			options.workerProfile = v
		case "--accept":
			v, err := needsValue()
			if err != nil {
				return options, err
			}
			options.acceptance = append(options.acceptance, v)
		default:
			return options, fmt.Errorf("unknown option %q", arg)
		}
	}
	if options.packetFile != "" && len(positional) > 0 {
		return options, errors.New("use --file or an inline job, not both")
	}
	if options.packetFile == "" && len(positional) == 0 {
		return options, errors.New("supply a job string or --file packet.md")
	}
	options.job = strings.Join(positional, " ")
	return options, nil
}

// cmdDo is the canonical front door. It creates an immutable local job packet
// and hands it to the existing work command; it is not another dispatcher.
func cmdDo() {
	options, err := parseDoArgs(os.Args[2:])
	if err != nil {
		fatalFrontDoor(err)
	}

	root := options.stateDir
	if root == "" {
		root = config.StateRoot()
	}
	root, _ = filepath.Abs(root)
	if parked(root) {
		fmt.Fprintf(os.Stderr, "GARY PARKED: new jobs are disabled at %s\n", filepath.Join(root, "PARKED"))
		os.Exit(2)
	}
	target, err := resolveRepo(options.repo)
	if err != nil {
		fatalFrontDoor(err)
	}
	ref := options.base
	if ref == "" {
		ref = gitOutput(target, "branch", "--show-current")
		if ref == "" {
			fatalFrontDoor(errors.New("cannot infer a branch; supply --base"))
		}
	}
	packetArgs := []string(nil)
	if options.packetFile == "" {
		packetArgs = []string{options.job}
	}
	packet, packetSource, err := readOriginalPacket(options.packetFile, packetArgs)
	if err != nil {
		fatalFrontDoor(err)
	}
	acceptance := options.acceptance
	if len(acceptance) == 0 {
		if configured := os.Getenv("GARY_ACCEPTANCE"); configured != "" {
			for _, command := range strings.Split(configured, "\n") {
				if strings.TrimSpace(command) != "" {
					acceptance = append(acceptance, command)
				}
			}
		}
	}
	if len(acceptance) == 0 {
		acceptance = append(acceptance, "git diff --check")
	}
	jobID := strings.TrimSpace(options.id)
	if jobID == "" {
		jobID = "job-" + time.Now().UTC().Format("20060102-150405.000000000")
	}
	if active := activeJob(root); active != "" && active != jobID {
		fatalFrontDoor(fmt.Errorf("another top-level job is active: %s", active))
	}
	jobDir := filepath.Join(root, "jobs", safeFrontName(jobID))
	if err := os.MkdirAll(jobDir, 0o700); err != nil {
		fatalFrontDoor(fmt.Errorf("create job directory: %w", err))
	}
	originalPath := filepath.Join(jobDir, "ORIGINAL_JOB.md")
	if err := writeNewFile(originalPath, []byte(packet)); err != nil {
		fatalFrontDoor(fmt.Errorf("preserve original packet: %w", err))
	}
	preflight := buildPreflight(jobID, packet, target, ref, strings.TrimSpace(options.workerProfile), acceptance)
	preflightPath := filepath.Join(jobDir, "PREFLIGHT.md")
	if err := writeNewFile(preflightPath, []byte(preflight)); err != nil {
		fatalFrontDoor(fmt.Errorf("preserve preflight: %w", err))
	}
	manifest := yardJobManifest{ID: jobID, RepoPath: target, BaseRef: ref, WorkerProfile: strings.TrimSpace(options.workerProfile), PacketPath: "ORIGINAL_JOB.md", Acceptance: acceptance, Preflight: preflight, StateDir: root}
	manifestPath := filepath.Join(jobDir, "job.json")
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		fatalFrontDoor(err)
	}
	if err := writeNewFile(manifestPath, data); err != nil {
		fatalFrontDoor(fmt.Errorf("write job manifest: %w", err))
	}
	fmt.Printf("%s\nPacket: %s\nPacket source: %s\nStarting.\n", preflight, originalPath, packetSource)
	os.Args = []string{"gary", "work", manifestPath, "--state-dir", root}
	cmdYardmaster()
}

func activeJob(root string) string {
	files, _ := filepath.Glob(filepath.Join(root, "jobs", "*.json"))
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var state struct {
			Job struct {
				ID string `json:"id"`
			} `json:"job"`
			Status string `json:"status"`
		}
		if json.Unmarshal(data, &state) == nil && (state.Status == "RUNNING" || state.Status == "NEEDS_MATTHEW") {
			return state.Job.ID
		}
	}
	return ""
}

func buildPreflight(jobID, packet, target, ref, profile string, acceptance []string) string {
	if profile == "" {
		profile = "gpt-5.6-luna (baseline)"
	}
	rationale := "baseline local Luna route is the only proven default"
	economics := "Incremental cash: UNKNOWN; allowance/quota: UNKNOWN; premium reserve: preserved"
	if strings.Contains(profile, "mimo") {
		rationale = "explicitly requested MiMo route; current evidence is successful execution with LOW identity confidence"
		economics = "Incremental cash: UNKNOWN; MiMo Token Plan allowance: included if available; quota figures: UNKNOWN"
	}
	if strings.Contains(profile, "antigravity") {
		rationale = "explicitly requested local Antigravity route; it remains candidate-only"
		economics = "Incremental cash: UNKNOWN; Antigravity included quota: UNKNOWN; quota figures: UNKNOWN"
	}
	if strings.Contains(profile, "/") && !strings.Contains(profile, "antigravity") {
		rationale = "explicitly requested provider/model route; it remains candidate-only"
		economics = "Incremental cash: UNKNOWN; provider allowance/billing: UNKNOWN; quota figures: UNKNOWN"
	}
	return fmt.Sprintf("GARY PREFLIGHT\n\nJob: %s\nTarget: %s\nBase: %s\nPacket bytes: %d\n\nPlan:\n1. Give the complete packet to the Luna Yardmaster.\n2. Select or honour the named worker route: %s.\n3. Run independent Git/worktree and declared acceptance checks.\n4. Integrate only an accepted clean candidate.\n\nWorker choice: %s\nWhy: %s\nExpected wall time: broad range, approximately 15–120 minutes (confidence: LOW; derived from task size only)\n%s\nIncluded quota/allowance: UNKNOWN unless freshly reported. Prepaid or expiring credits: UNKNOWN. Premium reserve use: none planned unless Luna records a reason.\nMatthew needed: probably not; only a real authority, credential, spend, CAPTCHA/2FA or product-decision boundary should stop the job.\nYou can leave: YES while a healthy worker runs; GARY will wait mechanically for an event.\nAcceptance: %s\n", jobID, target, ref, len([]byte(packet)), profile, profile, rationale, economics, strings.Join(acceptance, " | "))
}

func cmdResume() {
	fs := flag.NewFlagSet("resume", flag.ExitOnError)
	answer := fs.String("answer", "", "Matthew's answer to the persisted human boundary")
	stateRoot := fs.String("state-dir", "", "canonical GARY state root")
	_ = fs.Parse(os.Args[2:])
	if fs.NArg() != 1 {
		fatalFrontDoor(errors.New("usage: gary resume <job-id> --answer \"...\""))
	}
	root := *stateRoot
	if root == "" {
		root = config.StateRoot()
	}
	manifestPath := filepath.Join(root, "jobs", safeFrontName(fs.Arg(0)), "job.json")
	if _, err := os.Stat(manifestPath); err != nil {
		fatalFrontDoor(fmt.Errorf("job %q not found under %s", fs.Arg(0), root))
	}
	os.Args = []string{"gary", "work", manifestPath, "--state-dir", root, "--answer", *answer}
	cmdYardmaster()
}

func cmdNeedsMatthew() {
	root := config.StateRoot()
	if value := os.Getenv("GARY_STATE_ROOT"); value != "" {
		root = value
	}
	files, _ := filepath.Glob(filepath.Join(root, "jobs", "**", "*.json"))
	if len(files) == 0 {
		files, _ = filepath.Glob(filepath.Join(root, "jobs", "*.json"))
	}
	found := false
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var state struct {
			Job struct {
				ID string `json:"id"`
			} `json:"job"`
			Status  string `json:"status"`
			Blocker *struct {
				Summary       string `json:"summary"`
				Needed        string `json:"needed"`
				Evidence      string `json:"evidence"`
				UnblockAction string `json:"unblock_action"`
			} `json:"blocker"`
		}
		if json.Unmarshal(data, &state) == nil && state.Status == "NEEDS_MATTHEW" && state.Blocker != nil {
			found = true
			fmt.Printf("GARY NEEDS MATTHEW\nJob: %s\nNeed: %s\nWhy: %s\nAction: %s\n\n", state.Job.ID, state.Blocker.Needed, state.Blocker.Evidence, state.Blocker.UnblockAction)
		}
	}
	if !found {
		fmt.Println("GARY NEEDS MATTHEW: none")
	}
}

func cmdPark() {
	root := config.StateRoot()
	if err := os.MkdirAll(root, 0o700); err != nil {
		fatalFrontDoor(err)
	}
	if err := writeNewFile(filepath.Join(root, "PARKED"), []byte("parked\n")); err != nil {
		fatalFrontDoor(err)
	}
	fmt.Printf("GARY PARKED\nState: %s\n", root)
}

func cmdUnpark() {
	root := config.StateRoot()
	if err := os.Remove(filepath.Join(root, "PARKED")); err != nil && !os.IsNotExist(err) {
		fatalFrontDoor(err)
	}
	fmt.Printf("GARY UNPARKED\nState: %s\n", root)
}

func resolveRepo(value string) (string, error) {
	if strings.TrimSpace(value) != "" {
		absolute, err := filepath.Abs(value)
		if err != nil {
			return "", err
		}
		if _, err := os.Stat(filepath.Join(absolute, ".git")); err != nil {
			return "", fmt.Errorf("repo %s is not a Git checkout", absolute)
		}
		return absolute, nil
	}
	root := gitOutput(".", "rev-parse", "--show-toplevel")
	if root == "" {
		return "", errors.New("not inside a Git repository; supply --repo")
	}
	return root, nil
}

func readOriginalPacket(path string, args []string) (string, string, error) {
	if path != "" && len(args) > 0 {
		return "", "", errors.New("use --file or an inline job, not both")
	}
	if path != "" {
		data, err := os.ReadFile(path)
		return string(data), path, err
	}
	if len(args) == 0 {
		return "", "", errors.New("supply a job string or --file packet.md")
	}
	return strings.Join(args, " "), "inline", nil
}

func writeNewFile(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(data)
	return err
}

func parked(root string) bool {
	_, err := os.Stat(filepath.Join(root, "PARKED"))
	return err == nil
}

func safeFrontName(value string) string {
	value = strings.TrimSpace(value)
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	if b.Len() == 0 {
		return "job"
	}
	return b.String()
}

func gitOutput(dir string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func fatalFrontDoor(err error) {
	fmt.Fprintf(os.Stderr, "GARY START FAILURE: %v\n", err)
	os.Exit(1)
}
