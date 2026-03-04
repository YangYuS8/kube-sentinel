package observability

import (
	"strings"
	"testing"
	"time"
)

func TestBuildReleaseReadinessSummaryDeterministicAndDedupe(t *testing.T) {
	now := time.Unix(100, 0)
	input := ReleaseReadinessInput{
		ActionType:   "restart",
		RiskLevel:    "HIGH",
		StrategyMode: "auto",
		CircuitTier:  "object",
		Decision:     "allow",
		OpenIncidents: []IncidentRecord{
			{ID: "inc-2", Source: "runtime", CreatedAt: now.Add(-time.Minute)},
			{ID: "inc-1", Source: "runtime", CreatedAt: now},
			{ID: "inc-1", Source: "runtime", CreatedAt: now},
		},
		RollbackCandidate: "rev-10",
		Drill: DrillAggregate{
			SuccessRate:          1.2,
			RollbackLatencyP95MS: -1,
			GateBypassCount:      3,
			RecentScore:          0.9,
		},
	}

	s1, err := BuildReleaseReadinessSummary(input)
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}
	s2, err := BuildReleaseReadinessSummary(input)
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}
	if s1.Serialize() != s2.Serialize() {
		t.Fatalf("expected deterministic serialized output")
	}
	if len(s1.OpenIncidents) != 2 {
		t.Fatalf("expected deduped incidents, got %d", len(s1.OpenIncidents))
	}
	if s1.OpenIncidents[0].ID != "inc-1" {
		t.Fatalf("expected latest incident first, got %s", s1.OpenIncidents[0].ID)
	}
	if s1.RiskLevel != "high" {
		t.Fatalf("expected normalized risk level high, got %s", s1.RiskLevel)
	}
	if s1.Drill.SuccessRate != 1 || s1.Drill.RollbackLatencyP95MS != 0 {
		t.Fatalf("expected score normalization applied")
	}
}

func TestBuildReleaseReadinessSummaryFallbackOnUnknownDecision(t *testing.T) {
	summary, err := BuildReleaseReadinessSummary(ReleaseReadinessInput{
		ActionType:        "",
		Decision:          "unexpected",
		RollbackCandidate: "",
	})
	if err == nil {
		t.Fatalf("expected build error for missing required evidence")
	}
	if summary.Decision != DecisionDegrade {
		t.Fatalf("expected fallback decision degrade, got %s", summary.Decision)
	}
	if summary.OnCallTemplate.Decision != DecisionDegrade {
		t.Fatalf("expected degrade template fallback")
	}
}

func TestMapOnCallTemplate(t *testing.T) {
	allow, ok := MapOnCallTemplate(DecisionAllow)
	if !ok || allow.AlertLevel != "info" {
		t.Fatalf("allow mapping mismatch")
	}
	degrade, ok := MapOnCallTemplate(DecisionDegrade)
	if !ok || degrade.ApprovalTrigger != "oncall-ack" {
		t.Fatalf("degrade mapping mismatch")
	}
	block, ok := MapOnCallTemplate(DecisionBlock)
	if !ok || !strings.Contains(block.RollbackAction, "rollback") {
		t.Fatalf("block mapping mismatch")
	}
	if _, ok := MapOnCallTemplate("xxx"); ok {
		t.Fatalf("unknown decision should not map")
	}
}

func TestEnforceProductionGate(t *testing.T) {
	summary := ReleaseReadinessSummary{Decision: DecisionAllow, RollbackCandidate: "rev-a"}
	decision, reason := summary.EnforceProductionGate(3)
	if decision != DecisionAllow || reason != "" {
		t.Fatalf("expected allow pass-through")
	}

	noCandidate := ReleaseReadinessSummary{Decision: DecisionAllow}
	decision, reason = noCandidate.EnforceProductionGate(3)
	if decision != DecisionBlock || reason != "release_readiness_missing_rollback_candidate" {
		t.Fatalf("expected rollback candidate block")
	}

	tooManyIncidents := ReleaseReadinessSummary{
		Decision:          DecisionAllow,
		RollbackCandidate: "rev-a",
		OpenIncidents:     []IncidentRecord{{ID: "i1"}, {ID: "i2"}, {ID: "i3"}, {ID: "i4"}},
	}
	decision, reason = tooManyIncidents.EnforceProductionGate(3)
	if decision != DecisionBlock || reason != "release_readiness_open_incidents_exceeded" {
		t.Fatalf("expected open incidents block")
	}
}

func TestBuildIncidentsFromCSV(t *testing.T) {
	now := time.Unix(1, 0)
	records := BuildIncidentsFromCSV("a, b, a", false, "runtime", now)
	if len(records) != 2 {
		t.Fatalf("expected deduped records")
	}
}

func TestParseDrillAggregateBoundaries(t *testing.T) {
	agg := ParseDrillAggregate("-1", "x", "2", "5")
	if agg.SuccessRate != 0 || agg.RollbackLatencyP95MS != 0 || agg.GateBypassCount != 2 || agg.RecentScore != 1 {
		t.Fatalf("unexpected drill aggregate normalization: %+v", agg)
	}
}
