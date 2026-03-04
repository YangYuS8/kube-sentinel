package safety

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSLOGovernancePolicyWithDefaults(t *testing.T) {
	policy := (SLOGovernancePolicy{}).WithDefaults()
	if policy.SampleWindowMinutes != 10 || policy.DegradeThresholdPercent != 60 || policy.BlockThresholdPercent != 90 {
		t.Fatalf("unexpected defaults: %+v", policy)
	}
	if err := policy.Validate(); err != nil {
		t.Fatalf("expected default policy valid: %v", err)
	}
}

func TestSLOGovernancePolicyValidateBoundaries(t *testing.T) {
	invalidWindow := DefaultSLOGovernancePolicy()
	invalidWindow.SampleWindowMinutes = 0
	if err := invalidWindow.Validate(); err == nil {
		t.Fatalf("expected invalid sample window to fail")
	}

	invalidThresholds := DefaultSLOGovernancePolicy()
	invalidThresholds.DegradeThresholdPercent = 95
	invalidThresholds.BlockThresholdPercent = 90
	if err := invalidThresholds.Validate(); err == nil {
		t.Fatalf("expected degrade >= block to fail")
	}
}

func TestLoadSLOGovernancePolicyFromFile(t *testing.T) {
	tempDir := t.TempDir()
	policyPath := filepath.Join(tempDir, "runtime-slo-policy.yaml")
	content := []byte("targetSuccessRatePercent: 98\nsampleWindowMinutes: 15\nerrorBudgetPercent: 8\ndegradeThresholdPercent: 55\nblockThresholdPercent: 85\n")
	if err := os.WriteFile(policyPath, content, 0o644); err != nil {
		t.Fatalf("write policy file failed: %v", err)
	}
	policy, err := LoadSLOGovernancePolicy(policyPath)
	if err != nil {
		t.Fatalf("load policy failed: %v", err)
	}
	if policy.SampleWindowMinutes != 15 || policy.BlockThresholdPercent != 85 {
		t.Fatalf("unexpected loaded policy: %+v", policy)
	}
}

func TestLoadSLOGovernancePolicyInvalidFile(t *testing.T) {
	tempDir := t.TempDir()
	policyPath := filepath.Join(tempDir, "runtime-slo-policy.yaml")
	content := []byte("sampleWindowMinutes: 0\ndegradeThresholdPercent: 90\nblockThresholdPercent: 80\n")
	if err := os.WriteFile(policyPath, content, 0o644); err != nil {
		t.Fatalf("write policy file failed: %v", err)
	}
	if _, err := LoadSLOGovernancePolicy(policyPath); err == nil {
		t.Fatalf("expected invalid policy file to fail")
	}
}

func TestEvaluateSLOBudgetAndIncidentMapping(t *testing.T) {
	policy := DefaultSLOGovernancePolicy()
	if got := EvaluateSLOBudget(policy, 20); got.Outcome != GateOutcomeAllow || got.BudgetStatus != "healthy" {
		t.Fatalf("unexpected allow evaluation: %+v", got)
	}
	if got := EvaluateSLOBudget(policy, 70); got.Outcome != GateOutcomeDegrade || got.BudgetStatus != "warning" {
		t.Fatalf("unexpected degrade evaluation: %+v", got)
	}
	if got := EvaluateSLOBudget(policy, 95); got.Outcome != GateOutcomeBlock || got.BudgetStatus != "exhausted" {
		t.Fatalf("unexpected block evaluation: %+v", got)
	}

	allowPlan := MapIncidentResponsePlan(GateOutcomeAllow)
	degradePlan := MapIncidentResponsePlan(GateOutcomeDegrade)
	blockPlan := MapIncidentResponsePlan(GateOutcomeBlock)
	if allowPlan.Severity != "info" || degradePlan.Severity != "warning" || blockPlan.Severity != "critical" {
		t.Fatalf("unexpected incident severities: allow=%+v degrade=%+v block=%+v", allowPlan, degradePlan, blockPlan)
	}
	if !blockPlan.ManualApprovalRequired {
		t.Fatalf("expected block plan to require manual approval")
	}
}

func TestValidateRolloutLayers(t *testing.T) {
	if err := ValidateRolloutLayers(nil); err == nil {
		t.Fatalf("expected empty rollout layers to fail")
	}

	layers := []SLORolloutLayer{
		{Name: "canary", Namespaces: []string{"default"}, StableWindowPassed: true},
		{Name: "baseline", Namespaces: []string{"prod"}},
	}
	if err := ValidateRolloutLayers(layers); err != nil {
		t.Fatalf("expected valid rollout layers: %v", err)
	}

	blocked := []SLORolloutLayer{
		{Name: "canary", Namespaces: []string{"default"}, StableWindowPassed: false},
		{Name: "baseline", Namespaces: []string{"prod"}},
	}
	if err := ValidateRolloutLayers(blocked); err == nil {
		t.Fatalf("expected blocked rollout progression to fail")
	}

	rollbackTriggered := []SLORolloutLayer{
		{Name: "canary", Namespaces: []string{"default"}, StableWindowPassed: true, RollbackConditionActive: true},
		{Name: "baseline", Namespaces: []string{"prod"}},
	}
	if err := ValidateRolloutLayers(rollbackTriggered); err == nil {
		t.Fatalf("expected rollback-active layer to block next layer")
	}
}

func TestShouldImmediateRollback(t *testing.T) {
	if ShouldImmediateRollback(SLORolloutLayer{RollbackConditionActive: false}) {
		t.Fatalf("expected no rollback when condition is false")
	}
	if !ShouldImmediateRollback(SLORolloutLayer{RollbackConditionActive: true}) {
		t.Fatalf("expected rollback when condition is true")
	}
}

func TestApplyThresholdChangeRequiresApprovalAndSupportsRollback(t *testing.T) {
	policy := DefaultSLOGovernancePolicy()
	observedAt := map[string]time.Time{}
	now := time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC)

	_, _, err := ApplyThresholdChange(policy, SLOThresholdChangeRequest{
		TargetObject:            "default/demo-app",
		DegradeThresholdPercent: 55,
		BlockThresholdPercent:   85,
		Approver:                "",
		Reason:                  "test",
	}, observedAt, now)
	if err == nil {
		t.Fatalf("expected missing approver to fail")
	}

	next, snapshot, err := ApplyThresholdChange(policy, SLOThresholdChangeRequest{
		TargetObject:            "default/demo-app",
		DegradeThresholdPercent: 55,
		BlockThresholdPercent:   85,
		Approver:                "oncall-a",
		Reason:                  "reduce noisy degrade",
	}, observedAt, now)
	if err != nil {
		t.Fatalf("expected approved threshold change to succeed: %v", err)
	}
	if next.DegradeThresholdPercent != 55 || next.BlockThresholdPercent != 85 {
		t.Fatalf("unexpected applied thresholds: %+v", next)
	}
	if snapshot.DegradeThresholdPercent != policy.DegradeThresholdPercent || snapshot.BlockThresholdPercent != policy.BlockThresholdPercent {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}

	rolledBack, err := RollbackThresholdChange(next, snapshot)
	if err != nil {
		t.Fatalf("expected rollback to succeed: %v", err)
	}
	if rolledBack.DegradeThresholdPercent != policy.DegradeThresholdPercent || rolledBack.BlockThresholdPercent != policy.BlockThresholdPercent {
		t.Fatalf("expected rollback to restore previous thresholds: %+v", rolledBack)
	}
}

func TestApplyThresholdChangeObservationWindow(t *testing.T) {
	policy := DefaultSLOGovernancePolicy()
	observedAt := map[string]time.Time{}
	now := time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC)

	_, _, err := ApplyThresholdChange(policy, SLOThresholdChangeRequest{
		TargetObject:            "default/demo-app",
		DegradeThresholdPercent: 58,
		BlockThresholdPercent:   88,
		Approver:                "oncall-a",
		Reason:                  "initial tuning",
	}, observedAt, now)
	if err != nil {
		t.Fatalf("expected first approved tuning to pass: %v", err)
	}

	_, _, err = ApplyThresholdChange(policy, SLOThresholdChangeRequest{
		TargetObject:            "default/demo-app",
		DegradeThresholdPercent: 57,
		BlockThresholdPercent:   87,
		Approver:                "oncall-b",
		Reason:                  "repeat tuning",
	}, observedAt, now.Add(2*time.Minute))
	if err == nil {
		t.Fatalf("expected repeated tuning inside observation window to fail")
	}

	_, _, err = ApplyThresholdChange(policy, SLOThresholdChangeRequest{
		TargetObject:            "default/demo-app",
		DegradeThresholdPercent: 57,
		BlockThresholdPercent:   87,
		Approver:                "oncall-b",
		Reason:                  "post window tuning",
	}, observedAt, now.Add(11*time.Minute))
	if err != nil {
		t.Fatalf("expected tuning after observation window to pass: %v", err)
	}
}
