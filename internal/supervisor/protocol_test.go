package supervisor

import (
	"testing"
	"time"
)

func TestDecisionNormalize(t *testing.T) {
	d := Decision{Action: ActionGuide, Guidance: "  keep the working parser; fix span selection  "}
	if err := d.Normalize(3, "review-7", time.Date(2026, 9, 2, 16, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if d.Generation != 3 || d.ReviewID != "review-7" || d.Guidance != "keep the working parser; fix span selection" {
		t.Fatalf("normalized decision = %+v", d)
	}
}

func TestDecisionGuideRequiresGuidance(t *testing.T) {
	d := Decision{Action: ActionGuide}
	if err := d.Normalize(1, "r", time.Now()); err == nil {
		t.Fatal("GUIDE without guidance should fail")
	}
}

func TestTelemetryValidate(t *testing.T) {
	ok := Telemetry{SchemaVersion: 1, JobID: "job", ReviewID: "r1", Phase: "running", Turn: 1}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := ok
	bad.Phase = "thinking"
	if err := bad.Validate(); err == nil {
		t.Fatal("unknown phase should fail")
	}
}
