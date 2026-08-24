package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/config"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/gemini"
)

func cmdAgentSmoke() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config load failed: %v\n", err)
		os.Exit(1)
	}

	if cfg.GeminiAPIKey == "" {
		fmt.Fprintln(os.Stderr, "no Gemini API key: set GEMINI_API_KEY environment variable")
		os.Exit(1)
	}

	agent := "antigravity-preview-05-2026"
	model := cfg.ImplementerModel

	fmt.Println("GARY // AGENT SMOKE")
	fmt.Println()
	fmt.Printf("Project    %s\n", cfg.GCPProject)
	fmt.Printf("Agent      %s\n", agent)
	fmt.Printf("Model      %s\n", model)
	fmt.Printf("Thinking   low\n")
	fmt.Println()

	client := gemini.NewClient(cfg.GeminiAPIKey)

	input, err := buildAgentInput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "build input: %v\n", err)
		os.Exit(1)
	}

	req := gemini.InteractionRequest{
		Agent: agent,
		Input: input,
		Environment: &gemini.Environment{
			Type: "remote",
		},
	}

	fmt.Println("WORKING")

	submittedAt := time.Now()

	resp, err := client.RunInteraction(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "interaction failed: %v\n", err)
		os.Exit(1)
	}

	finishedAt := time.Now()

	if resp.Status != "completed" {
		fmt.Fprintf(os.Stderr, "interaction status: %s\n", resp.Status)
		os.Exit(1)
	}

	fmt.Println("COMPLETED")
	fmt.Println()
	fmt.Println("DOWNLOADING RESULT")

	if resp.EnvironmentID == "" {
		fmt.Fprintln(os.Stderr, "no environment ID returned")
		os.Exit(1)
	}

	// Query environment metadata for size telemetry
	envInfo, envErr := client.GetEnvironment(resp.EnvironmentID)
	var envSizeBytes int64
	if envErr == nil && envInfo.SizeBytes != "" {
		fmt.Sscanf(envInfo.SizeBytes, "%d", &envSizeBytes)
	}

	// Download with bounded retry for 404
	var snapshotReader io.ReadCloser
	var downloadErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
		snapshotReader, _, downloadErr = client.DownloadEnvironment(resp.EnvironmentID)
		if downloadErr == nil {
			break
		}
		// If 404, check if environment still exists
		if strings.Contains(downloadErr.Error(), "404") {
			_, checkErr := client.GetEnvironment(resp.EnvironmentID)
			if checkErr != nil {
				fmt.Fprintf(os.Stderr, "environment not accessible: %v\n", checkErr)
				break
			}
		}
	}
	if downloadErr != nil {
		fmt.Fprintf(os.Stderr, "download failed: %v\n", downloadErr)
		os.Exit(1)
	}
	defer snapshotReader.Close()

	downloadStart := time.Now()
	extractDir, err := extractSnapshot(snapshotReader)
	downloadDuration := time.Since(downloadStart)
	if err != nil {
		fmt.Fprintf(os.Stderr, "extract failed: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(extractDir)

	// Record snapshot telemetry
	fmt.Printf("  Environment reported size: %d bytes\n", envSizeBytes)
	fmt.Printf("  Snapshot extracted in:     %s\n", downloadDuration)

	taskDir := filepath.Join(extractDir, "workspace", "task")

	fmt.Println()
	fmt.Println("VERIFYING")

	markerVerified := verifyMarker(taskDir)
	testsUnchanged := verifyTestsUnchanged(taskDir)
	localTestsPassed := runLocalTests(taskDir)

	fmt.Println()
	fmt.Println("RECORDING RESULT")

	result := AgentSmokeResult{
		Kind:            "gary-agent-smoke",
		Project:         cfg.GCPProject,
		Agent:           agent,
		Model:           model,
		ThinkingLevel:   "low",
		MaxTotalTokens:  250000,
		InteractionID:   resp.ID,
		EnvironmentID:   resp.EnvironmentID,
		Status:          resp.Status,
		SubmittedAt:     submittedAt.Format(time.RFC3339),
		FinishedAt:      finishedAt.Format(time.RFC3339),
		DurationSeconds: finishedAt.Sub(submittedAt).Seconds(),
		ServiceTier:     resp.ServiceTier,
		ActualModel:     resp.Model,
		EnvSizeBytes:    envSizeBytes,
	}

	if resp.Usage != nil {
		result.TotalInputTokens = resp.Usage.TotalInputTokens
		result.TotalOutputTokens = resp.Usage.TotalOutputTokens
		result.TotalThoughtTokens = resp.Usage.TotalThoughtTokens
		result.TotalCachedTokens = resp.Usage.TotalCachedTokens
		result.TotalToolUseTokens = resp.Usage.TotalToolUseTokens
		result.TotalTokens = resp.Usage.TotalTokens
	}

	result.MarkerVerified = markerVerified
	result.TestsUnchanged = testsUnchanged
	result.LocalTestsPassed = localTestsPassed

	if err := saveAgentSmokeResult(cfg.ResultsDir, result); err != nil {
		fmt.Fprintf(os.Stderr, "save result: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	if markerVerified && testsUnchanged && localTestsPassed {
		fmt.Println("AGENT SMOKE PASSED")
	} else {
		fmt.Println("AGENT SMOKE FAILED")
		if !markerVerified {
			fmt.Println("  marker not verified")
		}
		if !testsUnchanged {
			fmt.Println("  tests were modified")
		}
		if !localTestsPassed {
			fmt.Println("  local tests failed")
		}
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("DONE")
	fmt.Printf("Duration    %.0fs\n", result.DurationSeconds)
	fmt.Printf("Tokens      %d (in: %d, out: %d, thought: %d)\n",
		result.TotalTokens, result.TotalInputTokens,
		result.TotalOutputTokens, result.TotalThoughtTokens)
}

func buildAgentInput() (string, error) {
	clampGo, err := os.ReadFile("testdata/agent-smoke/task/clamp.go")
	if err != nil {
		return "", fmt.Errorf("read clamp.go: %w", err)
	}
	clampTest, err := os.ReadFile("testdata/agent-smoke/task/clamp_test.go")
	if err != nil {
		return "", fmt.Errorf("read clamp_test.go: %w", err)
	}
	goMod, err := os.ReadFile("testdata/agent-smoke/task/go.mod")
	if err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	agentsMd, err := os.ReadFile("AGENTS.md")
	if err != nil {
		return "", fmt.Errorf("read AGENTS.md: %w", err)
	}
	rulebook, err := os.ReadFile("GARY_RULEBOOK.md")
	if err != nil {
		return "", fmt.Errorf("read GARY_RULEBOOK.md: %w", err)
	}

	return fmt.Sprintf(`You are a GARY implementation worker.

Read AGENTS.md and GARY_RULEBOOK.md.

Work only on /workspace/task.

Fix the implementation so all existing tests pass.

Do not modify the tests.

Do not redesign the task.

Run go test ./...

If Go is unavailable, install only what is necessary to run the tests.

After tests pass, create:

/workspace/task/GARY_AGENT_RESULT.txt

containing exactly:

GARY_AGENT_SMOKE_OK

Then stop.

Here are the files you need to create:

=== /workspace/task/go.mod ===
%s

=== /workspace/task/clamp.go ===
%s

=== /workspace/task/clamp_test.go ===
%s

=== /workspace/AGENTS.md ===
%s

=== /workspace/GARY_RULEBOOK.md ===
%s`, goMod, clampGo, clampTest, agentsMd, rulebook), nil
}

func extractSnapshot(reader io.Reader) (string, error) {
	dir, err := os.MkdirTemp("", "gary-agent-snapshot-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	// Stream directly from HTTP response to tar.Reader (plain TAR, not gzip)
	tr := tar.NewReader(reader)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			os.RemoveAll(dir)
			return "", fmt.Errorf("tar next: %w", err)
		}

		name := filepath.Clean(header.Name)
		if strings.HasPrefix(name, "/") || strings.Contains(name, "..") {
			continue
		}

		target := filepath.Join(dir, name)

		// Reject paths that escape the extraction directory
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dir)) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				os.RemoveAll(dir)
				return "", fmt.Errorf("mkdir: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				os.RemoveAll(dir)
				return "", fmt.Errorf("mkdir parent: %w", err)
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				os.RemoveAll(dir)
				return "", fmt.Errorf("create file: %w", err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				os.RemoveAll(dir)
				return "", fmt.Errorf("write file: %w", err)
			}
			f.Close()
		case tar.TypeSymlink:
			// Reject symlinks pointing outside extraction dir
			link := header.Linkname
			if strings.HasPrefix(link, "/") || strings.Contains(link, "..") {
				continue
			}
		}
	}

	return dir, nil
}

func verifyMarker(taskDir string) bool {
	data, err := os.ReadFile(filepath.Join(taskDir, "GARY_AGENT_RESULT.txt"))
	if err != nil {
		fmt.Println("  MARKER MISSING")
		return false
	}
	if strings.TrimSpace(string(data)) == "GARY_AGENT_SMOKE_OK" {
		fmt.Println("  MARKER VERIFIED")
		return true
	}
	fmt.Printf("  MARKER WRONG: %q\n", strings.TrimSpace(string(data)))
	return false
}

func verifyTestsUnchanged(taskDir string) bool {
	original, err := os.ReadFile("testdata/agent-smoke/task/clamp_test.go")
	if err != nil {
		fmt.Println("  CANNOT READ ORIGINAL TESTS")
		return false
	}
	extracted, err := os.ReadFile(filepath.Join(taskDir, "clamp_test.go"))
	if err != nil {
		fmt.Println("  CANNOT READ EXTRACTED TESTS")
		return false
	}
	if bytes.Equal(original, extracted) {
		fmt.Println("  TESTS UNCHANGED")
		return true
	}
	fmt.Println("  TESTS WERE MODIFIED")
	return false
}

func runLocalTests(taskDir string) bool {
	command := exec.Command("go", "test", "./...")
	command.Dir = taskDir
	out, err := command.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if err != nil {
		fmt.Printf("  LOCAL TESTS FAILED: %s\n", output)
		return false
	}
	fmt.Println("  LOCAL TESTS PASS")
	return true
}

// AgentSmokeResult holds the result of an agent smoke test.
type AgentSmokeResult struct {
	Kind               string  `json:"kind"`
	Project            string  `json:"project"`
	Agent              string  `json:"agent"`
	Model              string  `json:"model"`
	ThinkingLevel      string  `json:"thinking_level"`
	MaxTotalTokens     int     `json:"max_total_tokens"`
	InteractionID      string  `json:"interaction_id"`
	EnvironmentID      string  `json:"environment_id"`
	Status             string  `json:"status"`
	SubmittedAt        string  `json:"submitted_at"`
	FinishedAt         string  `json:"finished_at"`
	DurationSeconds    float64 `json:"duration_seconds"`
	TotalInputTokens   int     `json:"total_input_tokens"`
	TotalOutputTokens  int     `json:"total_output_tokens"`
	TotalThoughtTokens int     `json:"total_thought_tokens"`
	TotalCachedTokens  int     `json:"total_cached_tokens"`
	TotalToolUseTokens int     `json:"total_tool_use_tokens"`
	TotalTokens        int     `json:"total_tokens"`
	ServiceTier        string  `json:"service_tier"`
	ActualModel        string  `json:"actual_model"`
	EnvSizeBytes       int64   `json:"env_size_bytes"`
	MarkerVerified     bool    `json:"marker_verified"`
	TestsUnchanged     bool    `json:"tests_unchanged"`
	LocalTestsPassed   bool    `json:"local_tests_passed"`
}

func saveAgentSmokeResult(resultsDir string, result AgentSmokeResult) error {
	dir := filepath.Join(resultsDir, "agent-smoke")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	filename := fmt.Sprintf("%s.json", result.InteractionID)
	path := filepath.Join(dir, filename)

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}
