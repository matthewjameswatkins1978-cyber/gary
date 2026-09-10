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

// cmdDo is the canonical front door. It creates an immutable local job packet
// and hands it to the existing work command; it is not another dispatcher.
func cmdDo() {
	fs := flag.NewFlagSet("do", flag.ExitOnError)
	packetFile := fs.String("file", "", "read the original job packet from a file")
	repo := fs.String("repo", "", "target repository (defaults to the current Git repository)")
	base := fs.String("base", "", "base ref (defaults to the current branch)")
	stateRoot := fs.String("state-dir", "", "canonical GARY state root")
	id := fs.String("id", "", "stable job id (defaults to a timestamped id)")
	workerProfile := fs.String("worker-profile", "", "exact worker profile required for this job")
	var acceptance stringList
	fs.Var(&acceptance, "accept", "trusted acceptance command; repeat for multiple commands")
	_ = fs.Parse(os.Args[2:])

	root := *stateRoot
	if root == "" {
		root = config.StateRoot()
	}
	root, _ = filepath.Abs(root)
	if parked(root) {
		fmt.Fprintf(os.Stderr, "GARY PARKED: new jobs are disabled at %s\n", filepath.Join(root, "PARKED"))
		os.Exit(2)
	}
	target, err := resolveRepo(*repo)
	if err != nil {
		fatalFrontDoor(err)
	}
	ref := *base
	if ref == "" {
		ref = gitOutput(target, "branch", "--show-current")
		if ref == "" {
			fatalFrontDoor(errors.New("cannot infer a branch; supply --base"))
		}
	}
	packet, packetSource, err := readOriginalPacket(*packetFile, fs.Args())
	if err != nil {
		fatalFrontDoor(err)
	}
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
	jobID := strings.TrimSpace(*id)
	if jobID == "" {
		jobID = "job-" + time.Now().UTC().Format("20060102-150405.000000000")
	}
	jobDir := filepath.Join(root, "jobs", safeFrontName(jobID))
	if err := os.MkdirAll(jobDir, 0o700); err != nil {
		fatalFrontDoor(fmt.Errorf("create job directory: %w", err))
	}
	originalPath := filepath.Join(jobDir, "ORIGINAL_JOB.md")
	if err := writeNewFile(originalPath, []byte(packet)); err != nil {
		fatalFrontDoor(fmt.Errorf("preserve original packet: %w", err))
	}
	manifest := yardJobManifest{ID: jobID, RepoPath: target, BaseRef: ref, WorkerProfile: strings.TrimSpace(*workerProfile), PacketPath: "ORIGINAL_JOB.md", Acceptance: acceptance, StateDir: root}
	manifestPath := filepath.Join(jobDir, "job.json")
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		fatalFrontDoor(err)
	}
	if err := writeNewFile(manifestPath, data); err != nil {
		fatalFrontDoor(fmt.Errorf("write job manifest: %w", err))
	}
	fmt.Printf("GARY PREFLIGHT\nJob: %s\nTarget: %s\nBase: %s\nPacket: %s\nPacket source: %s\nAcceptance: %s\nStarting.\n", jobID, target, ref, originalPath, packetSource, strings.Join(acceptance, " | "))
	os.Args = []string{"gary", "work", manifestPath, "--state-dir", root}
	cmdYardmaster()
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
