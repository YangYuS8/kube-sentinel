package observability

import (
	"testing"
	"time"
)

func TestMetricsIncrement(t *testing.T) {
	m := &Metrics{}
	m.IncTriggers()
	m.IncSuccess()
	m.IncFailures()
	m.IncRollbacks()
	m.IncCircuitBreaks()
	m.IncMaintenanceWindowConflicts()
	m.IncSuppressed()
	m.IncReadOnlyBlocks("gate", "Deployment")
	m.IncStatefulSetControlledAction("StatefulSet", "restart", "blocked", "frozen")
	m.IncStatefulSetFreezeTriggers()
	m.IncStatefulSetL2Result("success")
	m.IncStatefulSetL2Result("fallback")
	m.IncStatefulSetL2Result("degraded")
	m.IncSnapshotCreateSuccess()
	m.IncSnapshotCreateFailure()
	m.IncSnapshotRestoreSuccess()
	m.IncSnapshotRestoreFailure()
	m.IncSnapshotCapacityBlock()
	m.AddSnapshotPruned(2)
	m.SetSnapshotActive(3)
	m.ObserveSnapshotRestoreDuration(2 * time.Second)
	m.ObserveStrategyDuration("process", time.Second)
	m.IncDeploymentL1Result("success")
	m.IncDeploymentL1Result("failed")
	m.IncDeploymentL1Result("blocked")
	m.IncDeploymentL2Result("success")
	m.IncDeploymentL2Result("fallback")
	m.IncDeploymentL2Result("degraded")
	m.IncDeploymentStageBlock("release_gate")
	m.IncProductionGateReport(true)
	m.IncProductionGateReport(false)
	l1Rate, l2Rate, l3Rate, blockRate := m.DeploymentTieredRates()
	if l1Rate <= 0 || l2Rate <= 0 || l3Rate <= 0 || blockRate <= 0 {
		t.Fatalf("expected positive deployment tiered rates")
	}
	if m.Triggers != 1 || m.Success != 1 || m.Failures != 1 || m.Rollbacks != 1 || m.CircuitBreaks != 1 || m.MaintenanceWindowConflicts != 1 || m.Suppressed != 1 || m.ReadOnlyBlocks != 1 || m.StatefulSetFreezeTriggers != 1 || m.StatefulSetL2Successes != 1 || m.StatefulSetL2Fallbacks != 1 || m.StatefulSetL2Degrades != 1 || m.SnapshotCreateSuccesses != 1 || m.SnapshotCreateFailures != 1 || m.SnapshotRestoreSuccesses != 1 || m.SnapshotRestoreFailures != 1 || m.SnapshotCapacityBlocks != 1 || m.SnapshotPruned != 2 || m.DeploymentL1Successes != 1 || m.DeploymentL1Failures != 1 || m.DeploymentL1Blocks != 1 || m.DeploymentL2Successes != 1 || m.DeploymentL2Fallbacks != 1 || m.DeploymentL2Degrades != 1 || m.DeploymentStageBlocks != 1 || m.ProductionGateReports != 2 || m.GateReportMissingFields != 1 {
		t.Fatalf("metrics counters not incremented")
	}
}

func TestAuditEventProductionGateReportCompleteness(t *testing.T) {
	complete := AuditEvent{
		GateResult:        "outcome=allow",
		RecoveryCondition: "success",
		Recommendation:    "continue observing",
	}
	if !complete.IsProductionGateReportComplete() {
		t.Fatalf("expected complete gate report")
	}

	incomplete := AuditEvent{GateResult: "outcome=block"}
	if incomplete.IsProductionGateReportComplete() {
		t.Fatalf("expected incomplete gate report")
	}
}

func TestAuditSinkWrite(t *testing.T) {
	s := &MemoryAuditSink{}
	s.Write(AuditEvent{ID: "1", Trigger: "alert"})
	if len(s.Events) != 1 {
		t.Fatalf("expected 1 audit event")
	}
}

func TestEventSinkRecord(t *testing.T) {
	s := &MemoryEventSink{}
	s.Record(RuntimeEvent{CorrelationKey: "trace-1", Reason: "GateBlocked"})
	if len(s.Events) != 1 {
		t.Fatalf("expected 1 runtime event")
	}
}
