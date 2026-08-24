package task

import "testing"

func TestClampBelowMin(t *testing.T) {
	got := Clamp(-5, 0, 10)
	if got != 0 {
		t.Errorf("Clamp(-5, 0, 10) = %d, want 0", got)
	}
}

func TestClampAboveMax(t *testing.T) {
	got := Clamp(15, 0, 10)
	if got != 10 {
		t.Errorf("Clamp(15, 0, 10) = %d, want 10", got)
	}
}

func TestClampInsideRange(t *testing.T) {
	got := Clamp(5, 0, 10)
	if got != 5 {
		t.Errorf("Clamp(5, 0, 10) = %d, want 5", got)
	}
}

func TestClampAtMinBoundary(t *testing.T) {
	got := Clamp(0, 0, 10)
	if got != 0 {
		t.Errorf("Clamp(0, 0, 10) = %d, want 0", got)
	}
}

func TestClampAtMaxBoundary(t *testing.T) {
	got := Clamp(10, 0, 10)
	if got != 10 {
		t.Errorf("Clamp(10, 0, 10) = %d, want 10", got)
	}
}

func TestClampNegativeRange(t *testing.T) {
	got := Clamp(0, -10, -5)
	if got != -5 {
		t.Errorf("Clamp(0, -10, -5) = %d, want -5", got)
	}
}

func TestClampAllEqual(t *testing.T) {
	got := Clamp(5, 5, 5)
	if got != 5 {
		t.Errorf("Clamp(5, 5, 5) = %d, want 5", got)
	}
}
