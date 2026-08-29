// Package gitops encapsulates worker repository checkout and result commits.
package gitops

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Clone clones a repository into the target directory.
func Clone(repoURL, targetDir string) error {
	return runGit(targetDir, "clone", repoURL, ".")
}

// CloneBranch clones one branch into the target directory.
func CloneBranch(repoURL, branch, targetDir string) error {
	return runGit(targetDir, "clone", "--branch", branch, "--single-branch", repoURL, ".")
}

// Checkout checks out an existing branch.
func Checkout(dir, branch string) error {
	return runGit(dir, "checkout", branch)
}

// CreateBranch creates and checks out a new branch from HEAD.
func CreateBranch(dir, branch string) error {
	return runGit(dir, "checkout", "-b", branch)
}

// Diff returns the unified diff of all uncommitted changes.
func Diff(dir string) (string, error) {
	return runGitCapture(dir, "diff", "HEAD")
}

// CommitSHA returns the short SHA of the current HEAD commit.
func CommitSHA(dir string) (string, error) {
	return runGitCapture(dir, "rev-parse", "--short", "HEAD")
}

// CommitAll stages all changes and commits with the given message.
func CommitAll(dir, message string) error {
	if err := runGit(dir, "add", "-A"); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	return runGit(dir,
		"-c", "user.name=GARY Worker",
		"-c", "user.email=gary@agent-yard.local",
		"commit", "-m", message,
	)
}

// Push pushes the branch to the remote.
func Push(dir, remote, branch string) error {
	return runGit(dir, "push", remote, branch)
}

// FetchBranch fetches a specific branch from the remote.
func FetchBranch(dir, remote, branch string) error {
	return runGit(dir, "fetch", remote, branch)
}

// FormatPatch generates a format-patch from baseBranch to HEAD.
func FormatPatch(dir, baseBranch string) (string, error) {
	return runGitCapture(dir, "format-patch", baseBranch, "--stdout")
}

// CherryPick applies one accepted worker commit to the current branch.
func CherryPick(dir, commit string) error {
	return runGit(dir, "cherry-pick", commit)
}

// AbortCherryPick aborts an in-progress cherry-pick.
func AbortCherryPick(dir string) error {
	return runGit(dir, "cherry-pick", "--abort")
}

// ApplyPatch applies a unified diff patch to the working tree.
func ApplyPatch(dir, patch string) error {
	cmd := gitCommand("git", "apply", "--verbose")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(patch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git apply: %w\n%s", err, string(out))
	}
	return nil
}

func runGit(dir string, args ...string) error {
	cmd := gitCommand("git", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runGitCapture(dir string, args ...string) (string, error) {
	cmd := gitCommand("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

// gitCommand supplies a short-lived GitHub App installation token to git
// without writing it to a remote URL or repository configuration. It is empty
// outside Actions, so local developer behaviour stays unchanged.
func gitCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	if token := os.Getenv("GARY_GITHUB_TOKEN"); token != "" {
		header := "Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte("x-access-token:"+token))
		cmd.Env = append(os.Environ(), "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=http.https://github.com/.extraheader", "GIT_CONFIG_VALUE_0="+header)
	}
	return cmd
}
