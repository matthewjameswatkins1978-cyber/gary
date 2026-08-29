package doctrine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeDoctrineStaysInDocumentedRulebook(t *testing.T) {
	path := filepath.Join("..", "..", "GARY_RULEBOOK.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	doc := strings.ReplaceAll(string(data), "\r\n", "\n")
	for name, rules := range map[string]string{
		"universal":  UniversalRules,
		"yardmaster": YardmasterRoleRules,
		"worker":     WorkerRoleRules,
	} {
		if !strings.Contains(doc, rules) {
			t.Fatalf("GARY_RULEBOOK.md must contain the exact runtime %s rules", name)
		}
	}
}
