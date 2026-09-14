package main

import (
	"strings"
	"testing"
	"time"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/blackbox"
)

func TestHistoryOptionsRejectMalformedInput(t *testing.T) {
	if _, _, err := parseHistoryOptions([]string{"--limit", "nope"}); err == nil {
		t.Fatal("malformed limit accepted")
	}
	if _, _, err := parseHistoryOptions([]string{"--unknown"}); err == nil {
		t.Fatal("unknown option accepted")
	}
	if _, err := blackbox.ParseSince("3x", time.Now()); err == nil {
		t.Fatal("malformed since accepted")
	}
}

func TestHistoryFilterMatchesOperationalFields(t *testing.T) {
	records := []blackbox.HistoryRecord{{JobID: "job-a", Repo: `C:\work\tethers`, Project: "tethers", Status: "FAILED", StartedAt: "2026-09-14T10:00:00Z", FailureCategories: []string{"worker-exit"}, Workers: []blackbox.Worker{{Profile: "mimo-v2.5"}}}, {JobID: "job-b", Repo: `C:\work\other`, Project: "other", Status: "DONE", StartedAt: "2026-09-14T10:00:00Z"}}
	options := historyOptions{repo: "tethers", worker: "mimo-v2.5", search: "worker-exit", limit: 10}
	filtered := filterRecords(records, options, time.Time{})
	if len(filtered) != 1 || filtered[0].JobID != "job-a" || !strings.Contains(recordText(filtered[0]), "worker-exit") {
		t.Fatalf("unexpected filter result: %+v", filtered)
	}
}
