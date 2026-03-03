package safety

import (
	"testing"
	"time"
)

func TestEvaluateMaintenanceWindow(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	d := Evaluate(GateInput{Now: now, MaintenanceWindows: []string{"11:00-13:00"}, MaxActions: 3})
	if d.Allow {
		t.Fatalf("expected blocked by maintenance window")
	}
}

func TestEvaluateRateLimit(t *testing.T) {
	d := Evaluate(GateInput{Now: time.Now(), ActionsInWindow: 3, MaxActions: 3})
	if d.Allow {
		t.Fatalf("expected blocked by rate limit")
	}
}

func TestEvaluateBlastRadiusBoundary(t *testing.T) {
	d := Evaluate(GateInput{Now: time.Now(), MaxActions: 3, AffectedPods: 11, ClusterPods: 100})
	if d.Allow {
		t.Fatalf("expected blocked by blast radius")
	}
}
