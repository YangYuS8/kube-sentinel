package scripts

import "testing"

func TestParseGateOutcome(t *testing.T) {
	tests := []struct {
		name       string
		phase      string
		block      string
		l2Result   string
		l2Decision string
		expected   string
	}{
		{name: "blocked by reason", phase: "Completed", block: "statefulset_readonly", expected: "block"},
		{name: "phase blocked", phase: "Blocked", expected: "block"},
		{name: "phase l3", phase: "L3", expected: "degrade"},
		{name: "degraded result", phase: "L2", l2Result: "degraded", expected: "degrade"},
		{name: "unknown phase defaults to degrade", phase: "UnknownPhase", expected: "degrade"},
		{name: "empty input defaults to degrade", expected: "degrade"},
		{name: "completed success", phase: "Completed", l2Result: "success", expected: "allow"},
		{name: "fallback unknown", phase: "PendingVerify", expected: "degrade"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ParseGateOutcome(test.phase, test.block, test.l2Result, test.l2Decision); got != test.expected {
				t.Fatalf("expected %s, got %s", test.expected, got)
			}
		})
	}
}

func TestValidateOutcomeCoverage(t *testing.T) {
	if err := ValidateOutcomeCoverage([]string{"allow", "block", "degrade"}); err != nil {
		t.Fatalf("expected full coverage: %v", err)
	}
	if err := ValidateOutcomeCoverage([]string{"allow", "UNKNOWN"}); err == nil {
		t.Fatalf("expected missing block to fail")
	}
	if err := ValidateOutcomeCoverage([]string{"allow", "block"}); err == nil {
		t.Fatalf("expected missing degrade to fail")
	}
}

func TestParseGateOutcomeIdempotent(t *testing.T) {
	first := ParseGateOutcome("", "", "", "")
	second := ParseGateOutcome("", "", "", "")
	if first != second {
		t.Fatalf("expected idempotent result, got %s then %s", first, second)
	}
	if first != "degrade" {
		t.Fatalf("expected safe default degrade, got %s", first)
	}
}

func TestValidatePrecommitCIConsistency(t *testing.T) {
	if err := ValidatePrecommitCIConsistency("allow", "allow"); err != nil {
		t.Fatalf("expected same outcomes to pass: %v", err)
	}
	if err := ValidatePrecommitCIConsistency("UNKNOWN", "degrade"); err != nil {
		t.Fatalf("expected normalized outcomes to pass: %v", err)
	}
	if err := ValidatePrecommitCIConsistency("", "allow"); err == nil {
		t.Fatalf("expected empty precommit outcome to fail")
	}
	if err := ValidatePrecommitCIConsistency("allow", "block"); err == nil {
		t.Fatalf("expected mismatched outcomes to fail")
	}
}

func TestValidateGateSLOConsistency(t *testing.T) {
	if err := ValidateGateSLOConsistency("allow", "allow"); err != nil {
		t.Fatalf("expected allow/allow to pass: %v", err)
	}
	if err := ValidateGateSLOConsistency("UNKNOWN", "degrade"); err != nil {
		t.Fatalf("expected normalized unknown/degrade to pass: %v", err)
	}
	if err := ValidateGateSLOConsistency("allow", "block"); err == nil {
		t.Fatalf("expected mismatch to fail")
	}
}

func TestMapIncidentAction(t *testing.T) {
	allow := MapIncidentAction("allow")
	degrade := MapIncidentAction("degrade")
	block := MapIncidentAction("block")
	if allow.Level != "info" || degrade.Level != "warning" || block.Level != "critical" {
		t.Fatalf("unexpected incident levels: allow=%+v degrade=%+v block=%+v", allow, degrade, block)
	}
	if block.Runbook == "" || block.RecoveryCondition == "" {
		t.Fatalf("expected block action to include runbook and recovery condition")
	}
}

func TestValidateRolloutStageProgression(t *testing.T) {
	err := ValidateRolloutStageProgression(RolloutStageEvidence{})
	if err == nil {
		t.Fatalf("expected empty rollout evidence to fail")
	}

	err = ValidateRolloutStageProgression(RolloutStageEvidence{
		CanaryStable:     true,
		RollbackHit:      true,
		TuningApproved:   true,
		RecoveryObserved: true,
	})
	if err != nil {
		t.Fatalf("expected complete rollout evidence to pass: %v", err)
	}
}

func TestValidatePostmortemEvidence(t *testing.T) {
	err := ValidatePostmortemEvidence(PostmortemEvidence{BreachReason: "budget_exhausted"})
	if err == nil {
		t.Fatalf("expected incomplete postmortem evidence to fail")
	}

	err = ValidatePostmortemEvidence(PostmortemEvidence{
		BreachReason:      "budget_exhausted",
		MitigationAction:  "rollback_to_canary",
		ThresholdDecision: "tighten_block_threshold",
		ObservationPlan:   "observe_30m_post_recovery",
	})
	if err != nil {
		t.Fatalf("expected complete postmortem evidence to pass: %v", err)
	}
}
