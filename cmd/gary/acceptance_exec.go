package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const maxAcceptanceCommandBytes = 4096

// runAcceptanceCommand executes trusted verifier code in the disposable
// acceptance workspace. Pipes, quoting, environment prefixes and command
// substitution are ordinary acceptance-test tools, so GARY deliberately uses
// the system shell rather than trying to reimplement Bash syntax.
func runAcceptanceCommand(dir, command, pythonBin string) ([]byte, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, errors.New("empty acceptance command")
	}

	env := os.Environ()
	if pythonBin != "" {
		venvBin := filepath.Dir(pythonBin)
		env = append(env, "PATH="+venvBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	}

	if bash, err := exec.LookPath("bash"); err == nil {
		cmd := exec.Command(bash, "-o", "pipefail", "-c", command)
		cmd.Dir = dir
		cmd.Env = env
		return cmd.CombinedOutput()
	}

	// GARY's hosted control plane has Bash. This fallback only exists for simple
	// local checks on hosts without it; complex checks fail clearly instead of
	// being silently misparsed.
	if strings.ContainsAny(command, "|&;<>()$`\'\\\"") {
		return nil, errors.New("acceptance command requires bash but bash is unavailable")
	}
	parts := acceptanceCommandParts(command, pythonBin)
	if len(parts) == 0 {
		return nil, errors.New("empty acceptance command")
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = dir
	cmd.Env = env
	return cmd.CombinedOutput()
}
