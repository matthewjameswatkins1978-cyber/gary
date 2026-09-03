package doctrine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeDoctrineIsACompactProjectionOfTheBible(t *testing.T) {
	path := filepath.Join("..", "..", "GARY_RULEBOOK.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	bible := strings.ReplaceAll(string(data), "\r\n", "\n")

	for _, marker := range []string{
		"Finish the real job.",
		"The Yardmaster owns **project completion**.",
		"Workers own bounded implementation outcomes.",
		"The Yardmaster May Fix Small Things",
		"Accepted child work is durable progress.",
		"Failure Belongs to the Stage That Failed",
		"Rules Must Earn Their Keep",
	} {
		if !strings.Contains(bible, marker) {
			t.Fatalf("GARY Bible missing canonical doctrine marker %q", marker)
		}
	}

	runtime := strings.Join([]string{CompletionPrinciple, UniversalRules, YardmasterRoleRules, WorkerRoleRules}, "\n")
	for _, marker := range []string{
		"finished, working, verified outcome",
		"trusted mechanical verification",
		"SMALL_FIX",
		"small, clearly bounded repairs",
		"Accepted child work is durable progress",
		"ACTIVE projects may plan, execute, verify, and integrate concurrently",
		"ACCEPTED and CANCELLED are terminal",
	} {
		if !strings.Contains(runtime, marker) {
			t.Fatalf("runtime doctrine lost Bible behaviour %q", marker)
		}
	}

	for _, superseded := range []string{
		"exactly one child job may run in a wave",
		"Once a project is ACCEPTED or ARCHIVED",
		"SMALL_FIX preserves the approach and returns exact narrow corrections to the Worker",
	} {
		if strings.Contains(runtime, superseded) {
			t.Fatalf("runtime doctrine retained superseded rule %q", superseded)
		}
	}
}
