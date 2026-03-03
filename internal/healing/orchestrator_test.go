package healing

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ksv1alpha1 "github.com/yangyus8/kube-sentinel/api/v1alpha1"
	"github.com/yangyus8/kube-sentinel/internal/observability"
	"github.com/yangyus8/kube-sentinel/internal/safety"
)

type fakeAdapter struct {
	supports   bool
	revisions  []RevisionRecord
	rollbackErr error
}

func (f fakeAdapter) Kind() string { return "Deployment" }
func (f fakeAdapter) Supports(kind string) bool { return f.supports && kind == "Deployment" }
func (f fakeAdapter) ListRevisions(_ context.Context, _, _ string) ([]RevisionRecord, error) { return f.revisions, nil }
func (f fakeAdapter) RollbackToRevision(_ context.Context, _, _, _ string) error { return f.rollbackErr }

func newReq() *ksv1alpha1.HealingRequest {
	return &ksv1alpha1.HealingRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "hr", Generation: 1},
		Spec: ksv1alpha1.HealingRequestSpec{Workload: ksv1alpha1.WorkloadRef{Kind: "Deployment", Namespace: "default", Name: "app"}},
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
		Adapter: fakeAdapter{supports: true, revisions: []RevisionRecord{{Revision: "1", UnixTime: 1, Healthy: false}}},
		Snapshotter: &MemorySnapshotter{},
		Breaker: safety.NewCircuitBreaker(3, 10, 1),
		Metrics: &observability.Metrics{},
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
		Adapter: fakeAdapter{supports: true, revisions: []RevisionRecord{{Revision: "2", UnixTime: 2, Healthy: true}}, rollbackErr: errors.New("rollback failed")},
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
		Adapter: fakeAdapter{supports: true, revisions: []RevisionRecord{{Revision: "2", UnixTime: 2, Healthy: true}}},
		Snapshotter: &MemorySnapshotter{},
		Breaker: safety.NewCircuitBreaker(3, 10, 1),
		Metrics: &observability.Metrics{},
		EventSink: events,
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
