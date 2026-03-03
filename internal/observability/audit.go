package observability

import "time"

type AuditEvent struct {
	ID          string
	Trigger     string
	Target      string
	BeforeState string
	AfterState  string
	Result      string
	CreatedAt   time.Time
}

type AuditSink interface {
	Write(event AuditEvent)
}

type MemoryAuditSink struct {
	Events []AuditEvent
}

func (m *MemoryAuditSink) Write(event AuditEvent) {
	m.Events = append(m.Events, event)
}
