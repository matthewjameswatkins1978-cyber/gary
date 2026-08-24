package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/config"
)

type check struct {
	name   string
	status string // "PASS", "FAIL", "WARN"
	detail string
}

func cmdDoctor() {
	fmt.Println("GARY // DOCTOR")
	fmt.Println()

	checks := runChecks()

	for _, c := range checks {
		fmt.Println(formatCheck(c))
	}

	fmt.Println()
	if doctorReady(checks) {
		fmt.Println("Result: READY")
	} else {
		fmt.Println("Result: NOT READY")
	}
}

func runChecks() []check {
	var checks []check

	checks = append(checks, checkGo())
	checks = append(checks, checkGofmt())
	checks = append(checks, checkGoVet())
	checks = append(checks, checkGopls())
	checks = append(checks, checkGoimports())
	checks = append(checks, checkStaticcheck())
	checks = append(checks, checkGit())
	checks = append(checks, checkDocker())
	checks = append(checks, checkGcloud())
	checks = append(checks, checkGitHubCLI())
	checks = append(checks, checkGoogleAccount())
	checks = append(checks, checkGCPProject())
	checks = append(checks, checkGCPRegion())
	checks = append(checks, checkGeminiAPIKey())
	checks = append(checks, checkConfig())

	return checks
}

func checkGo() check {
	out, err := runCmd("go", "version")
	if err != nil {
		return check{"Go", "FAIL", "not found on PATH"}
	}
	return check{"Go", "PASS", strings.TrimSpace(out)}
}

func checkGofmt() check {
	_, err := runCmd("gofmt", "-h")
	// gofmt -h returns exit code 0 on some systems, non-zero on others
	// The key is that the command exists
	if err != nil {
		// Check if it's a "not found" error vs just usage help
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "not recognized") {
			return check{"gofmt", "WARN", "not found (built into Go, check PATH)"}
		}
	}
	return check{"gofmt", "PASS", "built-in"}
}

func checkGoVet() check {
	_, err := runCmd("go", "vet", "-h")
	// go vet -h returns exit code 2 (usage), but that means it exists
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "not recognized") {
			return check{"go vet", "WARN", "not found (built into Go, check PATH)"}
		}
	}
	return check{"go vet", "PASS", "built-in"}
}

func checkGopls() check {
	out, err := runCmd("gopls", "version")
	if err != nil {
		return check{"gopls", "WARN", "not installed (optional, needed for LSP support)"}
	}
	return check{"gopls", "PASS", strings.TrimSpace(out)}
}

func checkGoimports() check {
	_, err := runCmd("goimports", "-h")
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "not recognized") {
			return check{"goimports", "WARN", "not installed (optional, helps with import organization)"}
		}
	}
	return check{"goimports", "PASS", "installed"}
}

func checkStaticcheck() check {
	out, err := runCmd("staticcheck", "-version")
	if err != nil {
		return check{"staticcheck", "WARN", "not installed (optional, advanced static analysis)"}
	}
	return check{"staticcheck", "PASS", strings.TrimSpace(out)}
}

func checkGit() check {
	out, err := runCmd("git", "--version")
	if err != nil {
		return check{"Git", "FAIL", "not found on PATH"}
	}
	return check{"Git", "PASS", strings.TrimSpace(out)}
}

func checkDocker() check {
	out, err := runCmd("docker", "--version")
	if err != nil {
		return check{"Docker", "WARN", "not found on PATH (not required for GCP managed builds)"}
	}
	return check{"Docker", "PASS", strings.TrimSpace(out)}
}

func checkGcloud() check {
	out, err := runCmd("gcloud", "--version")
	if err != nil {
		return check{"gcloud", "FAIL", "not found on PATH"}
	}
	first := strings.SplitN(strings.TrimSpace(out), "\n", 2)[0]
	return check{"gcloud", "PASS", first}
}

func checkGitHubCLI() check {
	out, err := runCmd("gh", "--version")
	if err != nil {
		return check{"GitHub CLI", "WARN", "gh not found on PATH"}
	}
	first := strings.SplitN(strings.TrimSpace(out), "\n", 2)[0]

	_, err = runCmd("gh", "auth", "status")
	if err != nil {
		return check{"GitHub CLI", "WARN", first + " (not authenticated: run gh auth login)"}
	}
	return check{"GitHub CLI", "PASS", first}
}

func checkGoogleAccount() check {
	out, err := runCmd("gcloud", "auth", "list", "--filter=status:ACTIVE", "--format=value(account)")
	if err != nil {
		return check{"Google account", "FAIL", "cannot query gcloud auth"}
	}
	acct := strings.TrimSpace(out)
	if acct == "" {
		return check{"Google account", "FAIL", "no active account (run: gcloud auth login)"}
	}
	return check{"Google account", "PASS", acct}
}

func checkGCPProject() check {
	out, err := runCmd("gcloud", "config", "get", "project")
	if err != nil {
		return check{"GCP project", "FAIL", "cannot query gcloud config"}
	}
	proj := strings.TrimSpace(out)
	if proj == "" || proj == "(unset)" {
		return check{"GCP project", "WARN", "no project set (run: gcloud config set project PROJECT_ID)"}
	}
	return check{"GCP project", "PASS", proj}
}

func checkGCPRegion() check {
	project := os.Getenv("GARY_GCP_PROJECT")
	region := os.Getenv("GARY_GCP_REGION")
	if project == "" || region == "" {
		return check{"GCP region", "WARN", "set GARY_GCP_PROJECT and GARY_GCP_REGION for cloud jobs"}
	}
	return check{"GCP region", "PASS", region}
}

func checkConfig() check {
	cfg, err := config.Load()
	if err != nil {
		return check{"Config", "FAIL", err.Error()}
	}

	var parts []string
	if cfg.GCPProject != "" {
		parts = append(parts, "project="+cfg.GCPProject)
	} else {
		parts = append(parts, "project unset")
	}
	if cfg.GCPRegion != "" {
		parts = append(parts, "region="+cfg.GCPRegion)
	} else {
		parts = append(parts, "region unset")
	}
	if cfg.GCPZone != "" {
		parts = append(parts, "zone="+cfg.GCPZone)
	}
	if cfg.GitHubRepo != "" {
		parts = append(parts, "repo="+cfg.GitHubRepo)
	}
	parts = append(parts, "results="+cfg.ResultsDir)

	return check{"Config", "PASS", strings.Join(parts, ", ")}
}

func checkGeminiAPIKey() check {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		return check{"Gemini API key", "FAIL", "set GEMINI_API_KEY environment variable"}
	}
	return check{"Gemini API key", "OK", "set"}
}

func runCmd(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func doctorReady(checks []check) bool {
	for _, c := range checks {
		if c.status == "FAIL" {
			return false
		}
	}
	return true
}

func formatCheck(c check) string {
	return fmt.Sprintf("%-5s %-22s %s", c.status, c.name, c.detail)
}
