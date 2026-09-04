package doctrine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeDoctrineMatchesSmallConstitution(t *testing.T) {
	path := filepath.Join("..", "..", "GARY_RULEBOOK.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	bible := string(data)
	for _, marker := range []string{
		"Give GARY the whole job",
		"Let the agent be an agent",
		"Model escalation is human-controlled",
		"Context is pull-based",
		"Whole job. One capable agent.",
	} {
		if !strings.Contains(bible, marker) {
			t.Fatalf("small constitution missing %q", marker)
		}
	}

	runtime := CompletionPrinciple + "\n" + UniversalRules + "\n" + WorkerRoleRules
	for _, marker := range []string{
		"Do not ask the human to do work you can reasonably do yourself",
		"ordinary tools, packages, SDKs",
		"model_escalation",
		"never an automatic action",
		"single GARY implementation agent for the whole supplied job",
	} {
		if !strings.Contains(runtime, marker) {
			t.Fatalf("runtime doctrine missing %q", marker)
		}
	}
	for _, obsolete := range []string{
		"SMALL_FIX",
		"Accepted child work",
		"ACTIVE projects may plan",
		"trusted mechanical verification passes, Yardmaster quality judgement",
		"manage sibling jobs",
	} {
		if strings.Contains(runtime, obsolete) {
			t.Fatalf("runtime doctrine retained orchestration doctrine %q", obsolete)
		}
	}
	if len(runtime) > 7000 {
		t.Fatalf("permanent runtime doctrine grew too large: %d bytes", len(runtime))
	}
}
