// Package gitops encapsulates worker repository checkout and result commits.
package gitops

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Clone clones a repository into the target directory.
func Clone(repoURL, targetDir string) error {
	return runGit(targetDir, "clone", repoURL, ".")
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
	return runGit(dir, "commit", "-m", message)
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

// ApplyPatch applies a unified diff patch to the working tree.
func ApplyPatch(dir, patch string) error {
	cmd := exec.Command("git", "apply", "--verbose")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(patch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git apply: %w\n%s", err, string(out))
	}
	return nil
}

func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runGitCapture(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}
