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
	if m.Triggers != 1 || m.Success != 1 || m.Failures != 1 || m.Rollbacks != 1 || m.CircuitBreaks != 1 || m.MaintenanceWindowConflicts != 1 || m.Suppressed != 1 || m.ReadOnlyBlocks != 1 || m.StatefulSetFreezeTriggers != 1 || m.StatefulSetL2Successes != 1 || m.StatefulSetL2Fallbacks != 1 || m.StatefulSetL2Degrades != 1 || m.SnapshotCreateSuccesses != 1 || m.SnapshotCreateFailures != 1 || m.SnapshotRestoreSuccesses != 1 || m.SnapshotRestoreFailures != 1 || m.SnapshotCapacityBlocks != 1 || m.SnapshotPruned != 2 {
		t.Fatalf("metrics counters not incremented")
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
