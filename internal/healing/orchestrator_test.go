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
	supports     bool
	revisions    []RevisionRecord
	listErr      error
	rollbackErr  error
	affectedPods int
	clusterPods  int
}

func (f fakeAdapter) Kind() string              { return "Deployment" }
func (f fakeAdapter) Supports(kind string) bool { return f.supports && kind == "Deployment" }
func (f fakeAdapter) ListRevisions(_ context.Context, _, _ string) ([]RevisionRecord, error) {
	return f.revisions, f.listErr
}
func (f fakeAdapter) RollbackToRevision(_ context.Context, _, _, _ string) error {
	return f.rollbackErr
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
