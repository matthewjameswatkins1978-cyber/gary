// Package forge provides deterministic conventional project bootstrap plans.
// It never decides product architecture; Yardmaster supplies the family and
// module/name, and Forge only creates the standard starting environment.
package forge

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Family string

const (
	SwiftIOS      Family = "swift-ios"
	RustCLI       Family = "rust-cli"
	GoCLI         Family = "go-cli"
	TypeScriptWeb Family = "typescript-web"
	PythonService Family = "python-service"
	Dotnet        Family = "dotnet"
	Generic       Family = "generic"
)

type Request struct {
	Family Family `json:"family"`
	Module string `json:"module,omitempty"`
	Name   string `json:"name,omitempty"`
}

type Plan struct {
	Family   Family            `json:"family"`
	Commands [][]string        `json:"commands"`
	Files    map[string][]byte `json:"files"`
}

func SupportedFamilies() []Family {
	return []Family{SwiftIOS, RustCLI, GoCLI, TypeScriptWeb, PythonService, Dotnet, Generic}
}

func BuildPlan(req Request) (Plan, error) {
	if req.Family == "" {
		return Plan{}, errors.New("project family is required")
	}
	valid := false
	for _, family := range SupportedFamilies() {
		if req.Family == family {
			valid = true
			break
		}
	}
	if !valid {
		return Plan{}, fmt.Errorf("unsupported project family %q", req.Family)
	}
	if req.Name == "" {
		req.Name = "project"
	}
	if req.Module == "" {
		req.Module = req.Name
	}
	plan := Plan{Family: req.Family, Files: map[string][]byte{
		"README.md": []byte("# " + req.Name + "\n\nBootstrapped by GARY Forge.\n"),
	}}
	switch req.Family {
	case SwiftIOS:
		plan.Commands = [][]string{{"swift", "package", "init", "--type", "executable"}}
	case RustCLI:
		plan.Commands = [][]string{{"cargo", "init", "--vcs", "none", "--name", req.Name}}
	case GoCLI:
		plan.Commands = [][]string{{"go", "mod", "init", req.Module}}
	case TypeScriptWeb:
		plan.Commands = [][]string{{"npm", "init", "-y"}}
	case PythonService:
		plan.Files["pyproject.toml"] = []byte("[build-system]\nrequires = [\"setuptools>=68\"]\nbuild-backend = \"setuptools.build_meta\"\n\n[project]\nname = \"" + strings.ToLower(req.Name) + "\"\nversion = \"0.1.0\"\n")
		plan.Files["src/"+strings.ToLower(req.Name)+"/__init__.py"] = []byte("")
		plan.Files["tests/test_smoke.py"] = []byte("def test_bootstrap():\n    assert True\n")
	case Dotnet:
		plan.Commands = [][]string{{"dotnet", "new", "console", "--force"}}
	case Generic:
		plan.Commands = [][]string{{"git", "init"}}
	}
	return plan, nil
}

// Bootstrap applies a plan without overwriting existing files. External
// standard-tool commands are run only after the safe baseline files are
// created; an existing repository is never deleted or reinitialised.
func Bootstrap(root string, req Request) (Plan, error) {
	plan, err := BuildPlan(req)
	if err != nil {
		return Plan{}, err
	}
	if strings.TrimSpace(root) == "" {
		return Plan{}, errors.New("forge root is required")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return Plan{}, err
	}
	for rel, data := range plan.Files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if _, err := os.Stat(path); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return Plan{}, err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return Plan{}, err
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return Plan{}, err
		}
	}
	for _, command := range plan.Commands {
		if command[0] == "git" && command[1] == "init" {
			if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
				continue
			}
		}
		cmd := exec.Command(command[0], command[1:]...)
		cmd.Dir = root
		if output, err := cmd.CombinedOutput(); err != nil {
			return Plan{}, fmt.Errorf("forge %s: %w: %s", strings.Join(command, " "), err, strings.TrimSpace(string(output)))
		}
	}
	return plan, nil
}
