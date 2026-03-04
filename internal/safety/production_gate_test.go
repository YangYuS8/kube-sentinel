package safety

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEvaluateProductionGatePriority(t *testing.T) {
	decision := EvaluateProductionGate(ProductionGateInput{
		MaintenanceWindowHit:     true,
		SecurityGateHit:          true,
		CircuitBreakerOpen:       true,
		ReleaseThresholdExceeded: true,
		SampleWindow:             "5m",
		KeyMetrics:               map[string]float64{"l1_success_rate": 70},
	})
	if decision.Outcome != GateOutcomeBlock {
		t.Fatalf("expected block outcome, got %s", decision.Outcome)
	}
	if decision.ReasonCode != ReasonCodeMaintenanceWindow {
		t.Fatalf("expected maintenance reason code, got %s", decision.ReasonCode)
	}
}

func TestEvaluateProductionGateCircuitBeforeRelease(t *testing.T) {
	decision := EvaluateProductionGate(ProductionGateInput{
		CircuitBreakerOpen:       true,
		ReleaseThresholdExceeded: true,
		SampleWindow:             "5m",
		KeyMetrics:               map[string]float64{"l2_success_rate": 40},
	})
	if decision.ReasonCode != ReasonCodeCircuitBreaker {
		t.Fatalf("expected circuit breaker reason, got %s", decision.ReasonCode)
	}
}

func TestEvaluateProductionGateDegradeForReleaseThreshold(t *testing.T) {
	decision := EvaluateProductionGate(ProductionGateInput{
		ReleaseThresholdExceeded: true,
		SampleWindow:             "10m",
		KeyMetrics:               map[string]float64{"block_rate": 45},
		Thresholds:               GateThresholdSnapshot{BlockRatePercent: 45},
	})
	if decision.Outcome != GateOutcomeDegrade {
		t.Fatalf("expected degrade outcome, got %s", decision.Outcome)
	}
	if !decision.Action.ReadOnly || !decision.Action.EmitAlert || !decision.Action.ManualIntervention {
		t.Fatalf("expected conservative action set")
	}
}

func TestEvaluateProductionGateFallbackToConservativeBlockOnIncompleteData(t *testing.T) {
	decision := EvaluateProductionGate(ProductionGateInput{
		ReleaseThresholdExceeded: true,
		SampleWindow:             "",
		KeyMetrics:               nil,
	})
	if decision.Outcome != GateOutcomeBlock {
		t.Fatalf("expected conservative block, got %s", decision.Outcome)
	}
	if decision.ReasonCode != ReasonCodeDataIncomplete {
		t.Fatalf("expected data incomplete reason, got %s", decision.ReasonCode)
	}
	if decision.Evidence.DataComplete {
		t.Fatalf("expected incomplete evidence")
	}
}

func TestGateEvidenceSerializationAndCompleteness(t *testing.T) {
	evidence := GateEvidence{
		ReasonCode:     ReasonCodeReleaseThreshold,
		SampleWindow:   "15m",
		KeyMetrics:     map[string]float64{"l3_degrade_rate": 51},
		Thresholds:     GateThresholdSnapshot{L3DegradeRatePercent: 51},
		Recommendation: "enter read-only evaluation",
	}
	raw, err := json.Marshal(evidence)
	if err != nil {
		t.Fatalf("marshal evidence failed: %v", err)
	}
	decoded := GateEvidence{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal evidence failed: %v", err)
	}
	decoded.Normalize()
	if !decoded.DataComplete {
		t.Fatalf("expected evidence to be complete")
	}

	incomplete := GateEvidence{ReasonCode: ReasonCodeReleaseThreshold}
	rawIncomplete, err := json.Marshal(incomplete)
	if err != nil {
		t.Fatalf("marshal incomplete evidence failed: %v", err)
	}
	decodedIncomplete := GateEvidence{}
	if err := json.Unmarshal(rawIncomplete, &decodedIncomplete); err != nil {
		t.Fatalf("unmarshal incomplete evidence failed: %v", err)
	}
	decodedIncomplete.Normalize()
	if decodedIncomplete.DataComplete {
		t.Fatalf("expected incomplete evidence to remain incomplete")
	}
}

func TestConservativeModeExecutorIdempotentApplyAndRollback(t *testing.T) {
	executor := NewConservativeModeExecutor()
	decision := EvaluateProductionGate(ProductionGateInput{
		ReleaseThresholdExceeded: true,
		SampleWindow:             "5m",
		KeyMetrics:               map[string]float64{"block_rate": 60},
	})
	state, changed, err := executor.Apply("ns/app", decision)
	if err != nil {
		t.Fatalf("apply conservative mode failed: %v", err)
	}
	if !changed {
		t.Fatalf("expected first apply to change state")
	}
	if !state.ReadOnly || !state.AlertEmitted || !state.ManualRequired {
		t.Fatalf("expected read-only alert manual flags to be true")
	}

	_, changed, err = executor.Apply("ns/app", decision)
	if err != nil {
		t.Fatalf("second apply failed: %v", err)
	}
	if changed {
		t.Fatalf("expected second apply with same decision to be idempotent")
	}

	newDecision := EvaluateProductionGate(ProductionGateInput{
		CircuitBreakerOpen: true,
		SampleWindow:       "5m",
		KeyMetrics:         map[string]float64{"circuit_open": 1},
	})
	_, changed, err = executor.Apply("ns/app", newDecision)
	if err != nil {
		t.Fatalf("apply updated decision failed: %v", err)
	}
	if !changed {
		t.Fatalf("expected different decision to update state")
	}

	restored, ok := executor.Rollback("ns/app")
	if !ok {
		t.Fatalf("expected rollback to restore previous state")
	}
	if restored.ReasonCode != decision.ReasonCode {
		t.Fatalf("expected rollback to restore previous reason code")
	}
}

func TestConservativeModeExecutorSnapshotRestoreAndEscalationGuard(t *testing.T) {
	executor := NewConservativeModeExecutor()
	if !executor.CanAutoEscalate("ns/app") {
		t.Fatalf("expected no state to allow auto escalation")
	}

	decision := EvaluateProductionGate(ProductionGateInput{
		ReleaseThresholdExceeded: true,
		SampleWindow:             "5m",
		KeyMetrics:               map[string]float64{"block_rate": 70},
	})
	if _, _, err := executor.Apply("ns/app", decision); err != nil {
		t.Fatalf("apply conservative mode failed: %v", err)
	}
	snapshot := executor.Snapshot("ns/app")
	if !snapshot.Exists {
		t.Fatalf("expected snapshot to capture current state")
	}
	if executor.CanAutoEscalate("ns/app") {
		t.Fatalf("expected manual-required state to block auto escalation")
	}

	executor.Restore("ns/app", ConservativeModeSnapshot{})
	if !executor.CanAutoEscalate("ns/app") {
		t.Fatalf("expected restore to empty snapshot to clear active block")
	}

	executor.Restore("ns/app", snapshot)
	if executor.CanAutoEscalate("ns/app") {
		t.Fatalf("expected restored snapshot to keep auto escalation blocked")
	}
}

func TestEvaluateProductionGateWithLegacyGateInput(t *testing.T) {
	now := time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC)
	decision := EvaluateProductionGate(ProductionGateInput{
		Gate: GateInput{
			Now:              now,
			ActionsInWindow:  3,
			MaxActions:       3,
			AffectedPods:     2,
			ClusterPods:      100,
			MaxPodPercentage: 10,
		},
		SampleWindow: "10m",
		KeyMetrics:   map[string]float64{"actions_in_window": 3},
	})
	if decision.Outcome != GateOutcomeDegrade {
		t.Fatalf("expected rate limit to produce degrade, got %s", decision.Outcome)
	}
	if decision.ReasonCode != ReasonCodeRateLimit {
		t.Fatalf("expected rate limit reason code, got %s", decision.ReasonCode)
	}
}
