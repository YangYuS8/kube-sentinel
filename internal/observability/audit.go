package observability

import "time"

type AuditEvent struct {
	ID                string
	Trigger           string
	Target            string
	WorkloadKind      string
	ActionType        string
	Phase             string
	SnapshotID        string
	Decision          string
	FreezeState       string
	BeforeState       string
	AfterState        string
	Result            string
	GateResult        string
	GateViolations    []string
	RecoveryCondition string
	Recommendation    string
	EvidenceComplete  bool
	CreatedAt         time.Time
}

func (e AuditEvent) IsProductionGateReportComplete() bool {
	if e.GateResult == "" {
		return false
	}
	if e.Recommendation == "" {
		return false
	}
	if e.RecoveryCondition == "" {
		return false
	}
	return true
}

type RuntimeEvent struct {
	CorrelationKey string
	SnapshotID     string
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
