package main

import (
	"strings"
	"testing"
)

func TestDoctorReady(t *testing.T) {
	tests := []struct {
		name   string
		checks []check
		want   bool
	}{
		{
			name:   "all pass",
			checks: []check{{"A", "PASS", ""}, {"B", "PASS", ""}},
			want:   true,
		},
		{
			name:   "warn is ok",
			checks: []check{{"A", "PASS", ""}, {"B", "WARN", "missing"}},
			want:   true,
		},
		{
			name:   "fail means not ready",
			checks: []check{{"A", "PASS", ""}, {"B", "FAIL", "broken"}},
			want:   false,
		},
		{
			name:   "multiple fails",
			checks: []check{{"A", "FAIL", ""}, {"B", "FAIL", ""}},
			want:   false,
		},
		{
			name:   "empty checks",
			checks: []check{},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := doctorReady(tt.checks); got != tt.want {
				t.Errorf("doctorReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatCheck(t *testing.T) {
	c := check{"Go", "PASS", "go1.26.7"}
	got := formatCheck(c)
	if !strings.Contains(got, "PASS") {
		t.Errorf("formatCheck missing status, got: %s", got)
	}
	if !strings.Contains(got, "Go") {
		t.Errorf("formatCheck missing name, got: %s", got)
	}
	if !strings.Contains(got, "go1.26.7") {
		t.Errorf("formatCheck missing detail, got: %s", got)
	}
}

func TestCheckStructFields(t *testing.T) {
	c := check{name: "Docker", status: "FAIL", detail: "not installed"}
	if c.name != "Docker" {
		t.Errorf("name = %q", c.name)
	}
	if c.status != "FAIL" {
		t.Errorf("status = %q", c.status)
	}
	if c.detail != "not installed" {
		t.Errorf("detail = %q", c.detail)
	}
}
