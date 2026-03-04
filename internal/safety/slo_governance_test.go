package safety

import (
	"os"
	"path/filepath"
	"testing"
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
