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

func TestValidateGateInputBoundaries(t *testing.T) {
	invalidCounts := ValidateGateInput(GateInput{MaxActions: 3, MaxPodPercentage: 10, AffectedPods: 11, ClusterPods: 10})
	if invalidCounts.Valid || invalidCounts.ReasonCode != "invalid_blast_radius_counts" {
		t.Fatalf("expected invalid blast radius counts reason, got %+v", invalidCounts)
	}

	invalidBlastRadius := ValidateGateInput(GateInput{MaxActions: 3, MaxPodPercentage: 0})
	if invalidBlastRadius.Valid || invalidBlastRadius.ReasonCode != "invalid_blast_radius" {
		t.Fatalf("expected invalid blast radius reason")
	}

	invalidRate := ValidateGateInput(GateInput{MaxActions: 0, MaxPodPercentage: 10})
	if invalidRate.Valid || invalidRate.ReasonCode != "invalid_rate_limit" {
		t.Fatalf("expected invalid rate limit reason")
	}
}
