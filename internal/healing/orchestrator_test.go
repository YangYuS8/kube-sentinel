package healing

import (
	"context"
	"errors"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ksv1alpha1 "github.com/yangyus8/kube-sentinel/api/v1alpha1"
	"github.com/yangyus8/kube-sentinel/internal/observability"
	"github.com/yangyus8/kube-sentinel/internal/safety"
)

type fakeAdapter struct {
	supports           bool
	revisions          []RevisionRecord
	listErr            error
	rollbackErr        error
	dependencyErr      error
	affectedPods       int
	clusterPods        int
	totalWorkloads     int
	unhealthyWorkloads int
}

func (f fakeAdapter) Kind() string              { return "Deployment" }
func (f fakeAdapter) Supports(kind string) bool { return f.supports && kind == "Deployment" }
func (f fakeAdapter) ListRevisions(_ context.Context, _, _ string) ([]RevisionRecord, error) {
	return f.revisions, f.listErr
}
func (f fakeAdapter) RollbackToRevision(_ context.Context, _, _, _ string) error {
	return f.rollbackErr
}
func (f fakeAdapter) ValidateRevisionDependencies(_ context.Context, _, _, _ string) error {
	return f.dependencyErr
}
func (f fakeAdapter) CountAffectedPods(_ context.Context, _, _ string) (int, error) {
	if f.affectedPods > 0 {
		return f.affectedPods, nil
	}
	return 1, nil
}
func (f fakeAdapter) CountClusterPods(_ context.Context, _ string) (int, error) {
	if f.clusterPods > 0 {
		return f.clusterPods, nil
	}
	return 100, nil
}
func (f fakeAdapter) CountTotalWorkloads(_ context.Context, _ string) (int, error) {
	if f.totalWorkloads > 0 {
		return f.totalWorkloads, nil
	}
	return 10, nil
}
func (f fakeAdapter) CountUnhealthyWorkloads(_ context.Context, _ string) (int, error) {
	if f.unhealthyWorkloads > 0 {
		return f.unhealthyWorkloads, nil
	}
	return 1, nil
}

func newReq() *ksv1alpha1.HealingRequest {
	return &ksv1alpha1.HealingRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "hr", Generation: 1},
		Spec:       ksv1alpha1.HealingRequestSpec{Workload: ksv1alpha1.WorkloadRef{Kind: "Deployment", Namespace: "default", Name: "app"}},
	}
}

func TestOrchestratorIdempotent(t *testing.T) {
	req := newReq()
	req.Status.Phase = ksv1alpha1.PhaseCompleted
	req.Status.ObservedGeneration = 1
	o := &Orchestrator{Adapter: fakeAdapter{supports: true}, Snapshotter: &MemorySnapshotter{}}
	if err := o.Process(context.Background(), req); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestOrchestratorUnsupportedKind(t *testing.T) {
	req := newReq()
	req.Spec.Workload.Kind = "StatefulSet"
	o := &Orchestrator{Adapter: fakeAdapter{supports: false}, Snapshotter: &MemorySnapshotter{}}
	if err := o.Process(context.Background(), req); err == nil {
		t.Fatalf("expected unsupported kind error")
	}
}

func TestOrchestratorL3OnNoHealthy(t *testing.T) {
	req := newReq()
	o := &Orchestrator{
		Adapter:     fakeAdapter{supports: true, revisions: []RevisionRecord{{Revision: "1", UnixTime: 1, Healthy: false}}, affectedPods: 1, clusterPods: 100},
		Snapshotter: &MemorySnapshotter{},
		Breaker:     safety.NewCircuitBreaker(3, 10, 1),
		Metrics:     &observability.Metrics{},
	}
	if err := o.Process(context.Background(), req); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if req.Status.Phase != ksv1alpha1.PhaseL3 {
		t.Fatalf("expected L3, got %s", req.Status.Phase)
	}
	if req.Status.LastEvidenceStatus != "insufficient-evidence" {
		t.Fatalf("expected insufficient-evidence, got %s", req.Status.LastEvidenceStatus)
	}
}

func TestOrchestratorRollbackFailureRestore(t *testing.T) {
	req := newReq()
	o := &Orchestrator{
		Adapter:     fakeAdapter{supports: true, revisions: []RevisionRecord{{Revision: "2", UnixTime: 2, Healthy: true}}, rollbackErr: errors.New("rollback failed"), affectedPods: 1, clusterPods: 100},
		Snapshotter: &MemorySnapshotter{},
	}
	if err := o.Process(context.Background(), req); err == nil {
		t.Fatalf("expected rollback error")
	}
}

func TestOrchestratorCorrelationAndEvent(t *testing.T) {
	req := newReq()
	req.Annotations = map[string]string{"kube-sentinel.io/correlation-key": "trace-1"}
	events := &observability.MemoryEventSink{}
	o := &Orchestrator{
		Adapter:     fakeAdapter{supports: true, revisions: []RevisionRecord{{Revision: "2", UnixTime: 2, Healthy: true}}, affectedPods: 1, clusterPods: 100},
		Snapshotter: &MemorySnapshotter{},
		Breaker:     safety.NewCircuitBreaker(3, 10, 1),
		Metrics:     &observability.Metrics{},
		EventSink:   events,
	}
	if err := o.Process(context.Background(), req); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if req.Status.CorrelationKey != "trace-1" {
		t.Fatalf("correlation key not propagated")
	}
	if len(events.Events) == 0 {
		t.Fatalf("expected runtime events")
	}
}

func TestOrchestratorGateUsesConfiguredBlastRadius(t *testing.T) {
	req := newReq()
	req.Spec.BlastRadius.MaxPodPercentage = 5
	o := &Orchestrator{
		Adapter:     fakeAdapter{supports: true, revisions: []RevisionRecord{{Revision: "2", UnixTime: 2, Healthy: true}}, affectedPods: 10, clusterPods: 100},
		Snapshotter: &MemorySnapshotter{},
	}
	err := o.Process(context.Background(), req)
	if err == nil {
		t.Fatalf("expected gate blocked due to blast radius config")
	}
	if req.Status.LastGateDecision == "" {
		t.Fatalf("expected gate evidence")
	}
}

func TestOrchestratorBreakerUsesConfiguredThreshold(t *testing.T) {
	req := newReq()
	req.Spec.CircuitBreaker.ObjectFailureThreshold = 1
	req.Spec.CircuitBreaker.DomainFailureThreshold = 100
	req.Spec.CircuitBreaker.CooldownMinutes = 10
	o := &Orchestrator{
		Adapter:     fakeAdapter{supports: true, listErr: errors.New("list failed"), affectedPods: 1, clusterPods: 100},
		Snapshotter: &MemorySnapshotter{},
		Now:         func() time.Time { return time.Unix(1000, 0) },
	}
	if err := o.Process(context.Background(), req); err == nil {
		t.Fatalf("expected first process to fail on revision list")
	}
	req2 := newReq()
	req2.Spec.CircuitBreaker.ObjectFailureThreshold = 1
	req2.Spec.CircuitBreaker.DomainFailureThreshold = 100
	req2.Spec.CircuitBreaker.CooldownMinutes = 10
	if err := o.Process(context.Background(), req2); err == nil {
		t.Fatalf("expected second process to be blocked by breaker")
	}
	if !req2.Status.CircuitBreaker.ObjectOpen && req2.Status.CircuitBreaker.OpenReason == "" {
		t.Fatalf("expected object breaker evidence")
	}
}

func TestOrchestratorPendingVerifyAndSuppressed(t *testing.T) {
	req := newReq()
	req.Annotations = map[string]string{
		"kube-sentinel.io/alert-status":   "firing",
		"kube-sentinel.io/alert-category": "CrashLoopBackOff",
		"kube-sentinel.io/alert-severity": "Critical",
	}
	now := time.Unix(1000, 0)
	o := &Orchestrator{
		Adapter:     fakeAdapter{supports: true, revisions: []RevisionRecord{{Revision: "2", UnixTime: 2, Healthy: true}}, affectedPods: 1, clusterPods: 100},
		Snapshotter: &MemorySnapshotter{},
		Now:         func() time.Time { return now },
	}
	if err := o.Process(context.Background(), req); err != nil {
		t.Fatalf("first process err: %v", err)
	}
	if req.Status.Phase != ksv1alpha1.PhasePendingVerify {
		t.Fatalf("expected pending verify phase, got %s", req.Status.Phase)
	}
	req.Annotations["kube-sentinel.io/alert-status"] = "resolved"
	if err := o.Process(context.Background(), req); err != nil {
		t.Fatalf("second process err: %v", err)
	}
	if req.Status.Phase != ksv1alpha1.PhaseSuppressed {
		t.Fatalf("expected suppressed phase, got %s", req.Status.Phase)
	}
}

func TestOrchestratorNamespaceBudgetBlocks(t *testing.T) {
	req := newReq()
	req.Spec.NamespaceBudget.BlockingThresholdPercent = 30
	req.Spec.NamespaceBudget.MinTotalWorkloads = 5
	req.Spec.NamespaceBudget.FallbackUnhealthyCount = 2
	req.Annotations = map[string]string{
		"kube-sentinel.io/alert-status": "firing",
	}
	now := time.Unix(1000, 0)
	o := &Orchestrator{
		Adapter:     fakeAdapter{supports: true, revisions: []RevisionRecord{{Revision: "2", UnixTime: 2, Healthy: true}}, affectedPods: 1, clusterPods: 100, totalWorkloads: 10, unhealthyWorkloads: 4},
		Snapshotter: &MemorySnapshotter{},
		Now:         func() time.Time { return now.Add(10 * time.Minute) },
	}
	req.Status.PendingSince = now.Format(time.RFC3339)
	req.Status.StableSampleCount = 3
	if err := o.Process(context.Background(), req); err == nil {
		t.Fatalf("expected namespace budget block")
	}
	if req.Status.ShadowAction == "" || req.Status.NamespaceBlockRate == 0 {
		t.Fatalf("expected namespace block evidence")
	}
}

func TestOrchestratorDependencyEvidenceFallback(t *testing.T) {
	req := newReq()
	req.Annotations = map[string]string{"kube-sentinel.io/alert-status": "firing"}
	now := time.Unix(1000, 0)
	o := &Orchestrator{
		Adapter:     fakeAdapter{supports: true, revisions: []RevisionRecord{{Revision: "2", UnixTime: 2, Healthy: true}}, dependencyErr: errors.New("missing config")},
		Snapshotter: &MemorySnapshotter{},
		Now:         func() time.Time { return now.Add(10 * time.Minute) },
	}
	req.Status.PendingSince = now.Format(time.RFC3339)
	req.Status.StableSampleCount = 3
	if err := o.Process(context.Background(), req); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if req.Status.Phase != ksv1alpha1.PhaseL3 {
		t.Fatalf("expected L3 fallback, got %s", req.Status.Phase)
	}
}

func TestOrchestratorSoakBoundary(t *testing.T) {
	req := newReq()
	req.Annotations = map[string]string{
		"kube-sentinel.io/alert-status":   "firing",
		"kube-sentinel.io/alert-category": "ProbeFailure",
		"kube-sentinel.io/alert-severity": "Medium",
	}
	start := time.Unix(1000, 0)
	now := start
	o := &Orchestrator{
		Adapter:     fakeAdapter{supports: true, revisions: []RevisionRecord{{Revision: "2", UnixTime: 2, Healthy: true}}, affectedPods: 1, clusterPods: 100},
		Snapshotter: &MemorySnapshotter{},
		Now:         func() time.Time { return now },
	}
	if err := o.Process(context.Background(), req); err != nil {
		t.Fatalf("first process err: %v", err)
	}
	if req.Status.Phase != ksv1alpha1.PhasePendingVerify {
		t.Fatalf("expected pending verify")
	}
	now = start.Add(30 * time.Second)
	if err := o.Process(context.Background(), req); err != nil {
		t.Fatalf("second process err: %v", err)
	}
	if req.Status.Phase != ksv1alpha1.PhasePendingVerify {
		t.Fatalf("expected still pending verify before soak boundary")
	}
	now = start.Add(3 * time.Minute)
	if err := o.Process(context.Background(), req); err != nil {
		t.Fatalf("third process err: %v", err)
	}
	if req.Status.Phase == ksv1alpha1.PhasePendingVerify {
		t.Fatalf("expected pending verify to finish after soak boundary")
	}
}

func TestOrchestratorNamespaceBudgetFallbackForSmallNamespace(t *testing.T) {
	req := newReq()
	req.Spec.NamespaceBudget.BlockingThresholdPercent = 30
	req.Spec.NamespaceBudget.MinTotalWorkloads = 5
	req.Spec.NamespaceBudget.FallbackUnhealthyCount = 2
	o := &Orchestrator{
		Adapter:     fakeAdapter{supports: true, totalWorkloads: 3, unhealthyWorkloads: 2},
		Snapshotter: &MemorySnapshotter{},
	}
	if err := o.Process(context.Background(), req); err == nil {
		t.Fatalf("expected small namespace fallback budget block")
	}
}

func TestOrchestratorEmergencyBypassOnce(t *testing.T) {
	req := newReq()
	req.Spec.NamespaceBudget.BlockingThresholdPercent = 30
	req.Spec.NamespaceBudget.MinTotalWorkloads = 5
	req.Spec.NamespaceBudget.FallbackUnhealthyCount = 2
	req.Spec.EmergencyTry.Enabled = true
	req.Spec.EmergencyTry.MaxAttempts = 1
	req.Annotations = map[string]string{"kube-sentinel.io/criticality": "high"}
	o := &Orchestrator{
		Adapter:     fakeAdapter{supports: true, revisions: []RevisionRecord{{Revision: "2", UnixTime: 2, Healthy: true}}, totalWorkloads: 10, unhealthyWorkloads: 4},
		Snapshotter: &MemorySnapshotter{},
	}
	if err := o.Process(context.Background(), req); err != nil {
		t.Fatalf("expected emergency bypass to allow processing: %v", err)
	}
	if req.Status.EmergencyAttempts != 1 {
		t.Fatalf("expected emergency attempt recorded")
	}
	req2 := newReq()
	req2.Spec.NamespaceBudget = req.Spec.NamespaceBudget
	req2.Spec.EmergencyTry = req.Spec.EmergencyTry
	req2.Status.EmergencyAttempts = 1
	req2.Annotations = map[string]string{"kube-sentinel.io/criticality": "high"}
	if err := o.Process(context.Background(), req2); err == nil {
		t.Fatalf("expected second emergency attempt to be blocked")
	}
}

func TestOrchestratorShadowActionEventAuditConsistency(t *testing.T) {
	req := newReq()
	events := &observability.MemoryEventSink{}
	audits := &observability.MemoryAuditSink{}
	o := &Orchestrator{
		Adapter:     fakeAdapter{supports: true, totalWorkloads: 10, unhealthyWorkloads: 4},
		Snapshotter: &MemorySnapshotter{},
		EventSink:   events,
		AuditSink:   audits,
	}
	if err := o.Process(context.Background(), req); err == nil {
		t.Fatalf("expected blocked by namespace budget")
	}
	if req.Status.ShadowAction == "" {
		t.Fatalf("expected shadow action in status")
	}
	if len(events.Events) == 0 || events.Events[len(events.Events)-1].Reason != "NamespaceBudgetBlocked" {
		t.Fatalf("expected namespace budget blocked event")
	}
	if len(audits.Events) == 0 || audits.Events[len(audits.Events)-1].Result != "blocked" {
		t.Fatalf("expected blocked audit event")
	}
}
