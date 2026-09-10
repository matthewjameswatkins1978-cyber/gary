package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/config"
)

// cmdWhere is intentionally mechanical. It helps Work/Codex discover the
// existing GARY installation without depending on the current branch or cwd.
func cmdWhere() {
	executable, err := os.Executable()
	if err != nil {
		executable = "unknown"
	}
	if absolute, absErr := filepath.Abs(executable); absErr == nil {
		executable = absolute
	}
	pathExecutable, pathErr := exec.LookPath("gary")
	if pathErr != nil {
		pathExecutable = "not found on PATH (invoke the installed canonical executable or add it to PATH)"
	}
	fmt.Printf("GARY EXECUTABLE: %s\nGARY ON PATH: %s\nGARY STATE ROOT: %s\n", executable, pathExecutable, config.StateRoot())
}
