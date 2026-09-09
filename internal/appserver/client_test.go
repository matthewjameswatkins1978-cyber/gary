package appserver

import "testing"

func TestDynamicToolsAreSmallAndUnique(t *testing.T) {
	tools := DynamicTools()
	if len(tools) != 5 {
		t.Fatalf("dynamic tool count=%d want 5", len(tools))
	}
	seen := map[string]bool{}
	for _, tool := range tools {
		name, ok := tool["name"].(string)
		if !ok || name == "" {
			t.Fatalf("tool has no name: %#v", tool)
		}
		if seen[name] {
			t.Fatalf("duplicate tool %q", name)
		}
		seen[name] = true
		if tool["type"] != "function" || tool["inputSchema"] == nil {
			t.Fatalf("tool %q is not a complete function declaration: %#v", name, tool)
		}
	}
}
