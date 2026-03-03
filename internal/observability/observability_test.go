package observability

import "testing"

func TestMetricsIncrement(t *testing.T) {
	m := &Metrics{}
	m.IncTriggers()
	m.IncSuccess()
	m.IncRollbacks()
	m.IncCircuitBreaks()
	if m.Triggers != 1 || m.Success != 1 || m.Rollbacks != 1 || m.CircuitBreaks != 1 {
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
