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
	d := Evaluate(GateInput{Now: time.Now(), MaxActions: 3, AffectedPods: 11, ClusterPods: 100, MaxPodPercentage: 10})
	if d.Allow {
		t.Fatalf("expected blocked by blast radius")
	}
}

func TestEvaluateBlastRadiusConfigurable(t *testing.T) {
	d := Evaluate(GateInput{Now: time.Now(), MaxActions: 3, AffectedPods: 11, ClusterPods: 100, MaxPodPercentage: 15})
	if !d.Allow {
		t.Fatalf("expected allow with larger blast radius threshold")
	}
}

func TestEvaluateInvalidConfig(t *testing.T) {
	d := Evaluate(GateInput{Now: time.Now(), MaxActions: 0, MaxPodPercentage: 10})
	if d.Allow || d.Reason != "invalid rate limit config" {
		t.Fatalf("expected invalid rate limit config")
	}
}
