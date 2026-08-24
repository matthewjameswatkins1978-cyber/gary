package main

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func cmdBenchmarkVerify() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: gary benchmark-verify <gcs-path> <job-id>")
		fmt.Fprintln(os.Stderr, "Example: gary benchmark-verify gs://gary-batch-results-418146339023/benchmarks/my-job/result.tar my-job")
		os.Exit(1)
	}

	gcsPath := os.Args[2]
	jobID := ""
	if len(os.Args) >= 4 {
		jobID = os.Args[3]
	} else {
		// Extract job ID from GCS path
		// gs://bucket/benchmarks/<job-id>/result.tar
		parts := strings.Split(gcsPath, "/")
		for i, p := range parts {
			if p == "benchmarks" && i+1 < len(parts) {
				jobID = parts[i+1]
				break
			}
		}
		if jobID == "" {
			jobID = "unknown"
		}
	}

	fmt.Println("GARY // BENCHMARK VERIFY")
	fmt.Println()
	fmt.Printf("GCS path: %s\n", gcsPath)
	fmt.Printf("Job ID:   %s\n", jobID)
	fmt.Println()

	// Download from GCS
	fmt.Println("DOWNLOADING")
	downloadStart := time.Now()
	tmpDir, err := downloadAndExtract(gcsPath)
	downloadDuration := time.Since(downloadStart)
	if err != nil {
		fmt.Fprintf(os.Stderr, "download/extract failed: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	fmt.Printf("  Downloaded and extracted in: %s\n", downloadDuration)

	// Find the task directory (clamp.go should be at the root of the extraction)
	taskDir := tmpDir

	// Check for result directory structure (may be nested)
	if _, err := os.Stat(filepath.Join(tmpDir, "result", "clamp.go")); err == nil {
		taskDir = filepath.Join(tmpDir, "result")
	} else if _, err := os.Stat(filepath.Join(tmpDir, "clamp.go")); err != nil {
		fmt.Fprintln(os.Stderr, "  clamp.go not found in extracted result")
		os.Exit(1)
	}

	// Get result size
	var resultSize int64
	filepath.Walk(taskDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			resultSize += info.Size()
		}
		return nil
	})
	fmt.Printf("  Result size: %d bytes\n", resultSize)

	fmt.Println()
	fmt.Println("VERIFYING")

	// Verify clamp_test.go is unchanged
	testsUnchanged := verifyTestsUnchanged(taskDir)

	// Verify marker
	markerVerified := verifyMarker(taskDir)

	// Run local tests
	localTestsPassed := runLocalTests(taskDir)

	fmt.Println()
	if markerVerified && testsUnchanged && localTestsPassed {
		fmt.Println("BENCHMARK VERIFY PASSED")
	} else {
		fmt.Println("BENCHMARK VERIFY FAILED")
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
	fmt.Printf("GCS path:      %s\n", gcsPath)
	fmt.Printf("Job ID:        %s\n", jobID)
	fmt.Printf("Result size:   %d bytes\n", resultSize)
	fmt.Printf("Download time: %s\n", downloadDuration)
}

func downloadAndExtract(gcsPath string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "gary-benchmark-verify-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	// Download TAR from GCS
	tarPath := filepath.Join(tmpDir, "result.tar")
	cmd := exec.Command("gsutil", "cp", gcsPath, tarPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("gsutil cp: %w\n%s", err, out)
	}

	// Open TAR file
	f, err := os.Open(tarPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("open tar: %w", err)
	}
	defer f.Close()

	// Extract TAR (safe path enforcement)
	tr := tar.NewReader(f)
	extractDir := filepath.Join(tmpDir, "extracted")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("create extract dir: %w", err)
	}

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			os.RemoveAll(tmpDir)
			return "", fmt.Errorf("tar next: %w", err)
		}

		name := filepath.Clean(header.Name)
		if strings.HasPrefix(name, "/") || strings.Contains(name, "..") {
			continue
		}

		target := filepath.Join(extractDir, name)

		// Reject paths that escape the extraction directory
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(extractDir)) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				os.RemoveAll(tmpDir)
				return "", fmt.Errorf("mkdir: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				os.RemoveAll(tmpDir)
				return "", fmt.Errorf("mkdir parent: %w", err)
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				os.RemoveAll(tmpDir)
				return "", fmt.Errorf("create file: %w", err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				os.RemoveAll(tmpDir)
				return "", fmt.Errorf("write file: %w", err)
			}
			f.Close()
		case tar.TypeSymlink:
			link := header.Linkname
			if strings.HasPrefix(link, "/") || strings.Contains(link, "..") {
				continue
			}
		}
	}

	return extractDir, nil
}
