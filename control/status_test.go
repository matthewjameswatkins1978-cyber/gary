package control

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestUpdateAndReadSnapshot(t *testing.T) {
	dir := t.TempDir()
	statusPath := filepath.Join(dir, "gary", "control", "status.json")

	// Write snapshot
	active := []JobEntry{
		{ID: "jc-01", State: JobRunning, Repo: "test-repo"},
	}
	recent := []JobEntry{
		{ID: "jc-00", State: JobAccepted, Summary: "done"},
	}

	err := UpdateSnapshot(dir, active, recent)
	if err != nil {
		t.Fatalf("UpdateSnapshot: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(statusPath); os.IsNotExist(err) {
		t.Fatalf("status.json not created at %s", statusPath)
	}

	// Read and verify
	snap, err := ReadSnapshot(dir)
	if err != nil {
		t.Fatalf("ReadSnapshot: %v", err)
	}

	if snap.YardState != YardRunning {
		t.Errorf("yard_state = %q, want %q", snap.YardState, YardRunning)
	}
	if len(snap.ActiveJobs) != 1 {
		t.Errorf("active_jobs len = %d, want 1", len(snap.ActiveJobs))
	}
	if snap.ActiveJobs[0].ID != "jc-01" {
		t.Errorf("active job ID = %q, want %q", snap.ActiveJobs[0].ID, "jc-01")
	}
	if len(snap.RecentJobs) != 1 {
		t.Errorf("recent_jobs len = %d, want 1", len(snap.RecentJobs))
	}
}

func TestBoundedHistory(t *testing.T) {
	dir := t.TempDir()

	// Create 25 recent jobs
	recent := make([]JobEntry, 25)
	for i := range recent {
		recent[i] = JobEntry{ID: "jc-" + string(rune('0'+i%10)), State: JobAccepted}
	}

	err := UpdateSnapshot(dir, nil, recent)
	if err != nil {
		t.Fatalf("UpdateSnapshot: %v", err)
	}

	snap, err := ReadSnapshot(dir)
	if err != nil {
		t.Fatalf("ReadSnapshot: %v", err)
	}

	if len(snap.RecentJobs) > maxRecentJobs {
		t.Errorf("recent_jobs len = %d, exceeds max %d", len(snap.RecentJobs), maxRecentJobs)
	}
}

func TestNoSecretsInSnapshot(t *testing.T) {
	dir := t.TempDir()

	active := []JobEntry{
		{ID: "jc-01", State: JobRunning, Summary: "normal work"},
	}

	err := UpdateSnapshot(dir, active, nil)
	if err != nil {
		t.Fatalf("UpdateSnapshot: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "gary", "control", "status.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// Should not contain any secret patterns
	content := string(data)
	badPatterns := []string{"GEMINI_API_KEY", "password", "secret", "token", "private_key"}
	for _, p := range badPatterns {
		if contains(content, p) {
			t.Errorf("snapshot contains forbidden pattern %q", p)
		}
	}
}

func TestSerializationRoundTrip(t *testing.T) {
	dir := t.TempDir()

	active := []JobEntry{
		{ID: "jc-01", State: JobRunning, EstCost: 0.0042},
	}
	recent := []JobEntry{
		{ID: "jc-00", State: JobAccepted, EstCost: 0.0031},
	}

	err := UpdateSnapshot(dir, active, recent)
	if err != nil {
		t.Fatalf("UpdateSnapshot: %v", err)
	}

	snap, err := ReadSnapshot(dir)
	if err != nil {
		t.Fatalf("ReadSnapshot: %v", err)
	}

	// Verify JSON is valid
	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var roundTrip Snapshot
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if roundTrip.YardState != snap.YardState {
		t.Errorf("round-trip yard_state mismatch: %q vs %q", roundTrip.YardState, snap.YardState)
	}
}

func TestYardStateTransitions(t *testing.T) {
	tests := []struct {
		name     string
		active   []JobEntry
		expected YardState
	}{
		{"parked with no jobs", nil, YardParked},
		{"running with active jobs", []JobEntry{{State: JobRunning}}, YardRunning},
		{"blocked with blocked job", []JobEntry{{State: JobRunning}, {State: JobBlocked}}, YardBlocked},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := computeYardState(tt.active)
			if state != tt.expected {
				t.Errorf("computeYardState = %q, want %q", state, tt.expected)
			}
		})
	}
}

func TestReadSnapshotNotExist(t *testing.T) {
	dir := t.TempDir()
	snap, err := ReadSnapshot(dir)
	if err != nil {
		t.Fatalf("ReadSnapshot on missing file: %v", err)
	}
	if snap.YardState != YardParked {
		t.Errorf("default yard_state = %q, want %q", snap.YardState, YardParked)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
