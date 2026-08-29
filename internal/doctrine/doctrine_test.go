package doctrine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUniversalForemanRulesStayInDocumentedRulebook(t *testing.T) {
	path := filepath.Join("..", "..", "GARY_RULEBOOK.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(data), UniversalForemanRules) {
		t.Fatal("GARY_RULEBOOK.md must contain the exact runtime universal foreman rules")
	}
}
