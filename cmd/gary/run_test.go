package main

import (
	"testing"
)

func TestSummarizeAcceptance(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "empty",
			input:  "",
			expect: "no-output",
		},
		{
			name:   "all pass",
			input:  "ok  \tclamp\t0.003s\nok  \tmath\t0.001s\n",
			expect: "ok  \tclamp\t0.003s\nok  \tmath\t0.001s",
		},
		{
			name:   "some fail",
			input:  "FAIL\tclamp\t0.004s\nok  \tmath\t0.001s\n",
			expect: "FAIL\tclamp\t0.004s\nok  \tmath\t0.001s",
		},
		{
			name:   "no ok lines",
			input:  "some random output\nmore output\n",
			expect: "some random output\nmore output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := summarizeAcceptance(tt.input)
			if got != tt.expect {
				t.Errorf("got %q, want %q", got, tt.expect)
			}
		})
	}
}

func TestCheckProgress(t *testing.T) {
	tests := []struct {
		name    string
		history []string
		current string
		expect  bool
	}{
		{
			name:    "first attempt always progress",
			history: []string{},
			current: "FAIL\tclamp",
			expect:  true,
		},
		{
			name:    "fewer failures is progress",
			history: []string{"FAIL\tclamp\nFAIL\tmath"},
			current: "FAIL\tclamp",
			expect:  true,
		},
		{
			name:    "same failures is no progress",
			history: []string{"FAIL\tclamp"},
			current: "FAIL\tclamp",
			expect:  false,
		},
		{
			name:    "more failures is no progress",
			history: []string{"FAIL\tclamp"},
			current: "FAIL\tclamp\nFAIL\tmath",
			expect:  false,
		},
		{
			name:    "pass after fail is progress",
			history: []string{"FAIL\tclamp"},
			current: "ok  \tclamp",
			expect:  true,
		},
		{
			name:    "different output is progress",
			history: []string{"FAIL\tclamp: wrong result"},
			current: "FAIL\tclamp: timeout",
			expect:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkProgress(tt.history, tt.current)
			if got != tt.expect {
				t.Errorf("checkProgress(%v, %q) = %v, want %v", tt.history, tt.current, got, tt.expect)
			}
		})
	}
}

func TestFormatRepairBrief(t *testing.T) {
	brief := &repairBrief{
		Cause:       "function returns input unchanged",
		Keep:        "test file and go.mod",
		Change:      "add clamping logic",
		DoNotChange: "tests",
		NextAction:  "implement min/max logic",
	}

	output := formatRepairBrief(brief)

	if output == "" {
		t.Fatal("formatRepairBrief returned empty string")
	}
	if !contains(output, "CAUSE: function returns input unchanged") {
		t.Errorf("output missing CAUSE: %s", output)
	}
	if !contains(output, "NEXT ACTION: implement min/max logic") {
		t.Errorf("output missing NEXT ACTION: %s", output)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
