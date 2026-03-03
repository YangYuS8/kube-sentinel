package observability

import "time"

type AuditEvent struct {
	ID           string
	Trigger      string
	Target       string
	WorkloadKind string
	BeforeState  string
	AfterState   string
	Result       string
	CreatedAt    time.Time
}

type RuntimeEvent struct {
	CorrelationKey string
	Namespace      string
	Name           string
	ResourceKind   string
	Reason         string
	Message        string
	Type           string
	CreatedAt      time.Time
}

type AuditSink interface {
	Write(event AuditEvent)
}

type EventSink interface {
	Record(event RuntimeEvent)
}

type MemoryAuditSink struct {
	Events []AuditEvent
}

type MemoryEventSink struct {
	Events []RuntimeEvent
}

func (m *MemoryAuditSink) Write(event AuditEvent) {
	m.Events = append(m.Events, event)
}

func (m *MemoryEventSink) Record(event RuntimeEvent) {
	m.Events = append(m.Events, event)
}
