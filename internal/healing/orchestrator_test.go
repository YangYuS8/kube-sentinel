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
	supports               bool
	revisions              []RevisionRecord
	listErr                error
	rollbackErr            error
	dependencyErr          error
	statefulSetActionErr   error
	statefulSetEvidenceErr error
	affectedPods           int
	clusterPods            int
	totalWorkloads         int
	unhealthyWorkloads     int
}

func (f fakeAdapter) Kind() string { return "Deployment" }
func (f fakeAdapter) Supports(kind string) bool {
	return f.supports && (kind == "Deployment" || kind == "StatefulSet")
}
func (f fakeAdapter) ListRevisions(_ context.Context, _, _ string) ([]RevisionRecord, error) {
	return f.revisions, f.listErr
}
func (f fakeAdapter) RollbackToRevision(_ context.Context, _, _, _ string) error {
	return f.rollbackErr
}
func (f fakeAdapter) ExecuteStatefulSetControlledAction(_ context.Context, _, _, _ string) error {
	return f.statefulSetActionErr
}
func (f fakeAdapter) ValidateStatefulSetEvidence(_ context.Context, _, _ string) error {
	return f.statefulSetEvidenceErr
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

type fakeRuntimeInputProvider struct {
	input RuntimeInput
	err   error
}

func (f fakeRuntimeInputProvider) Build(_ context.Context, _ *ksv1alpha1.HealingRequest) (RuntimeInput, error) {
	if f.err != nil {
		return RuntimeInput{}, f.err
	}
	return f.input, nil
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
	req.Spec.Workload.Kind = "Job"
	o := &Orchestrator{Adapter: fakeAdapter{supports: true}, Snapshotter: &MemorySnapshotter{}}
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

func TestOrchestratorStatefulSetReadOnlyBlocked(t *testing.T) {
	req := newReq()
	req.Spec.Workload.Kind = "StatefulSet"
	req.Annotations = map[string]string{
		"kube-sentinel.io/alert-status": "firing",
	}
	events := &observability.MemoryEventSink{}
	audits := &observability.MemoryAuditSink{}
	now := time.Unix(1000, 0)
	o := &Orchestrator{
		Adapter:     fakeAdapter{supports: true},
		Snapshotter: &MemorySnapshotter{},
		EventSink:   events,
		AuditSink:   audits,
		Now:         func() time.Time { return now.Add(10 * time.Minute) },
	}
	req.Status.PendingSince = now.Format(time.RFC3339)
	req.Status.StableSampleCount = 3
	if err := o.Process(context.Background(), req); err == nil {
		t.Fatalf("expected statefulset readonly block")
	}
	if req.Status.WorkloadCapability != "read-only" {
		t.Fatalf("expected read-only workload capability")
	}
	if req.Status.BlockReasonCode != "statefulset_readonly" {
		t.Fatalf("expected statefulset_readonly reason code")
	}
	if req.Status.ShadowAction == "" || req.Status.LastAction != "manual-intervention" {
		t.Fatalf("expected shadow action and manual intervention")
	}
	if req.Status.StatefulSetAuthorization == "" {
		t.Fatalf("expected statefulset authorization evidence")
	}
	if len(events.Events) == 0 || events.Events[len(events.Events)-1].Reason != "StatefulSetReadOnlyBlocked" {
		t.Fatalf("expected readonly blocked runtime event")
	}
	if len(audits.Events) == 0 || audits.Events[len(audits.Events)-1].WorkloadKind != "StatefulSet" {
		t.Fatalf("expected audit workload kind statefulset")
	}
}

func TestOrchestratorStatefulSetControlledActionAuthorized(t *testing.T) {
	req := newReq()
	req.Spec.Workload.Kind = "StatefulSet"
	req.Spec.StatefulSetPolicy.Enabled = true
	req.Spec.StatefulSetPolicy.ReadOnlyOnly = false
	req.Spec.StatefulSetPolicy.ControlledActionsEnabled = true
	req.Spec.StatefulSetPolicy.L2RollbackEnabled = true
	req.Spec.StatefulSetPolicy.RequireEvidence = false
	req.Spec.StatefulSetPolicy.ApprovalAnnotation = "kube-sentinel.io/statefulset-approved"
	req.Spec.StatefulSetPolicy.AllowedNamespaces = []string{"default"}
	req.Annotations = map[string]string{
		"kube-sentinel.io/statefulset-approved": "true",
	}
	o := &Orchestrator{
		Adapter:     fakeAdapter{supports: true},
		Snapshotter: &MemorySnapshotter{},
		RuntimeInputProvider: fakeRuntimeInputProvider{input: RuntimeInput{
			AffectedPods:       1,
			ClusterPods:        100,
			TotalWorkloads:     10,
			UnhealthyWorkloads: 1,
		}},
		Metrics: &observability.Metrics{},
	}
	if err := o.Process(context.Background(), req); err != nil {
		t.Fatalf("expected statefulset controlled action success: %v", err)
	}
	if req.Status.Phase != ksv1alpha1.PhaseCompleted {
		t.Fatalf("expected completed phase")
	}
	if req.Status.LastAction != "statefulset-controlled-restart" {
		t.Fatalf("expected controlled restart action")
	}
	if req.Status.WorkloadCapability != "conditional-writable" {
		t.Fatalf("expected conditional-writable capability")
	}
}

func TestOrchestratorStatefulSetControlledActionFailureFreeze(t *testing.T) {
	req := newReq()
	req.Spec.Workload.Kind = "StatefulSet"
	req.Spec.StatefulSetPolicy.Enabled = true
	req.Spec.StatefulSetPolicy.ReadOnlyOnly = false
	req.Spec.StatefulSetPolicy.ControlledActionsEnabled = true
	req.Spec.StatefulSetPolicy.L2RollbackEnabled = true
	req.Spec.StatefulSetPolicy.RequireEvidence = false
	req.Spec.StatefulSetPolicy.ApprovalAnnotation = "kube-sentinel.io/statefulset-approved"
	req.Spec.StatefulSetPolicy.AllowedNamespaces = []string{"default"}
	req.Spec.StatefulSetPolicy.FreezeWindowMinutes = 5
	req.Annotations = map[string]string{
		"kube-sentinel.io/statefulset-approved": "true",
	}
	now := time.Unix(1000, 0)
	o := &Orchestrator{
		Adapter: fakeAdapter{
			supports:             true,
			statefulSetActionErr: errors.New("restart failed"),
			revisions:            []RevisionRecord{{Revision: "rev-a", UnixTime: 10, Healthy: true}},
			rollbackErr:          errors.New("rollback failed"),
		},
		Snapshotter: &MemorySnapshotter{},
		RuntimeInputProvider: fakeRuntimeInputProvider{input: RuntimeInput{
			AffectedPods:       1,
			ClusterPods:        100,
			TotalWorkloads:     10,
			UnhealthyWorkloads: 1,
		}},
		Metrics: &observability.Metrics{},
		Now:     func() time.Time { return now },
	}
	if err := o.Process(context.Background(), req); err == nil {
		t.Fatalf("expected statefulset controlled action failure")
	}
	if req.Status.StatefulSetFreezeState != "frozen" || req.Status.StatefulSetFreezeUntil == "" {
		t.Fatalf("expected freeze state after failure")
	}
	if req.Status.StatefulSetL2Result != "fallback" {
		t.Fatalf("expected l2 fallback result")
	}
	req2 := newReq()
	req2.Spec.Workload = req.Spec.Workload
	req2.Spec.StatefulSetPolicy = req.Spec.StatefulSetPolicy
	req2.Annotations = req.Annotations
	req2.Status.StatefulSetFreezeState = "frozen"
	req2.Status.StatefulSetFreezeUntil = now.Add(2 * time.Minute).Format(time.RFC3339)
	if err := o.Process(context.Background(), req2); err == nil {
		t.Fatalf("expected frozen window block")
	}
	if req2.Status.BlockReasonCode != "statefulset_frozen" {
		t.Fatalf("expected statefulset_frozen block code")
	}
}

func TestOrchestratorStatefulSetL2RollbackSuccessAfterL1Failure(t *testing.T) {
	req := newReq()
	req.Spec.Workload.Kind = "StatefulSet"
	req.Spec.StatefulSetPolicy.Enabled = true
	req.Spec.StatefulSetPolicy.ReadOnlyOnly = false
	req.Spec.StatefulSetPolicy.ControlledActionsEnabled = true
	req.Spec.StatefulSetPolicy.L2RollbackEnabled = true
	req.Spec.StatefulSetPolicy.RequireEvidence = false
	req.Spec.StatefulSetPolicy.ApprovalAnnotation = "kube-sentinel.io/statefulset-approved"
	req.Spec.StatefulSetPolicy.AllowedNamespaces = []string{"default"}
	req.Annotations = map[string]string{"kube-sentinel.io/statefulset-approved": "true"}
	o := &Orchestrator{
		Adapter: fakeAdapter{
			supports:             true,
			statefulSetActionErr: errors.New("restart failed"),
			revisions:            []RevisionRecord{{Revision: "rev-ok", UnixTime: 100, Healthy: true}},
		},
		Snapshotter:          &MemorySnapshotter{},
		RuntimeInputProvider: fakeRuntimeInputProvider{input: RuntimeInput{AffectedPods: 1, ClusterPods: 100, TotalWorkloads: 10, UnhealthyWorkloads: 1}},
		Metrics:              &observability.Metrics{},
	}
	if err := o.Process(context.Background(), req); err != nil {
		t.Fatalf("expected l2 rollback success after l1 failure: %v", err)
	}
	if req.Status.Phase != ksv1alpha1.PhaseCompleted || req.Status.StatefulSetL2Result != "success" {
		t.Fatalf("expected phase completed with l2 success")
	}
	if req.Status.LastHealthyRevision != "rev-ok" {
		t.Fatalf("expected l2 selected healthy revision")
	}
}

func TestOrchestratorStatefulSetL2DegradeWhenNoCandidate(t *testing.T) {
	req := newReq()
	req.Spec.Workload.Kind = "StatefulSet"
	req.Spec.StatefulSetPolicy.Enabled = true
	req.Spec.StatefulSetPolicy.ReadOnlyOnly = false
	req.Spec.StatefulSetPolicy.ControlledActionsEnabled = true
	req.Spec.StatefulSetPolicy.L2RollbackEnabled = true
	req.Spec.StatefulSetPolicy.RequireEvidence = false
	req.Spec.StatefulSetPolicy.ApprovalAnnotation = "kube-sentinel.io/statefulset-approved"
	req.Spec.StatefulSetPolicy.AllowedNamespaces = []string{"default"}
	req.Annotations = map[string]string{"kube-sentinel.io/statefulset-approved": "true"}
	o := &Orchestrator{
		Adapter: fakeAdapter{
			supports:             true,
			statefulSetActionErr: errors.New("restart failed"),
			revisions:            []RevisionRecord{{Revision: "rev-bad", UnixTime: 10, Healthy: false}},
		},
		Snapshotter:          &MemorySnapshotter{},
		RuntimeInputProvider: fakeRuntimeInputProvider{input: RuntimeInput{AffectedPods: 1, ClusterPods: 100, TotalWorkloads: 10, UnhealthyWorkloads: 1}},
	}
	if err := o.Process(context.Background(), req); err != nil {
		t.Fatalf("expected l2 no-candidate degrade without hard error: %v", err)
	}
	if req.Status.Phase != ksv1alpha1.PhaseL3 || req.Status.StatefulSetL2Result != "degraded" {
		t.Fatalf("expected l3 degraded when no healthy candidate")
	}
}

func TestOrchestratorStatefulSetL2IdempotencyBlocked(t *testing.T) {
	req := newReq()
	req.Spec.Workload.Kind = "StatefulSet"
	req.Spec.StatefulSetPolicy.Enabled = true
	req.Spec.StatefulSetPolicy.ReadOnlyOnly = false
	req.Spec.StatefulSetPolicy.ControlledActionsEnabled = true
	req.Spec.StatefulSetPolicy.L2RollbackEnabled = true
	req.Spec.StatefulSetPolicy.RequireEvidence = false
	req.Spec.StatefulSetPolicy.ApprovalAnnotation = "kube-sentinel.io/statefulset-approved"
	req.Spec.StatefulSetPolicy.AllowedNamespaces = []string{"default"}
	req.Annotations = map[string]string{"kube-sentinel.io/statefulset-approved": "true"}
	now := time.Unix(2000, 0)
	o := &Orchestrator{
		Adapter: fakeAdapter{
			supports:             true,
			statefulSetActionErr: errors.New("restart failed"),
			revisions:            []RevisionRecord{{Revision: "rev-ok", UnixTime: 100, Healthy: true}},
		},
		Snapshotter:          &MemorySnapshotter{},
		RuntimeInputProvider: fakeRuntimeInputProvider{input: RuntimeInput{AffectedPods: 1, ClusterPods: 100, TotalWorkloads: 10, UnhealthyWorkloads: 1}},
		Now:                  func() time.Time { return now },
	}
	o.actionHistory = map[string][]time.Time{req.Spec.Workload.Namespace + "/" + req.Spec.Workload.Name + "/l2": {now.Add(-time.Minute)}}
	if err := o.Process(context.Background(), req); err == nil {
		t.Fatalf("expected l2 idempotency block")
	}
	if req.Status.BlockReasonCode != "statefulset_l2_idempotency_window" {
		t.Fatalf("expected l2 idempotency block reason")
	}
}

func TestOrchestratorStatefulSetAuthorizationFailure(t *testing.T) {
	req := newReq()
	req.Spec.Workload.Kind = "StatefulSet"
	req.Spec.StatefulSetPolicy.Enabled = true
	req.Spec.StatefulSetPolicy.ReadOnlyOnly = false
	req.Spec.StatefulSetPolicy.ControlledActionsEnabled = true
	req.Spec.StatefulSetPolicy.RequireEvidence = true
	req.Spec.StatefulSetPolicy.ApprovalAnnotation = "kube-sentinel.io/statefulset-approved"
	req.Spec.StatefulSetPolicy.AllowedNamespaces = []string{"default"}
	req.Annotations = map[string]string{}
	o := &Orchestrator{
		Adapter:     fakeAdapter{supports: true},
		Snapshotter: &MemorySnapshotter{},
		RuntimeInputProvider: fakeRuntimeInputProvider{input: RuntimeInput{
			AffectedPods:       1,
			ClusterPods:        100,
			TotalWorkloads:     10,
			UnhealthyWorkloads: 1,
		}},
	}
	if err := o.Process(context.Background(), req); err == nil {
		t.Fatalf("expected authorization failure")
	}
	if req.Status.BlockReasonCode != "statefulset_authorization_failed" {
		t.Fatalf("expected authorization failed reason code")
	}
}

func TestOrchestratorStatefulSetGateBoundaries(t *testing.T) {
	t.Run("maintenance window", func(t *testing.T) {
		req := newReq()
		req.Spec.Workload.Kind = "StatefulSet"
		req.Spec.MaintenanceWindows = []string{"00:00-23:59"}
		now := time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC)
		o := &Orchestrator{
			Adapter:     fakeAdapter{supports: true},
			Snapshotter: &MemorySnapshotter{},
			RuntimeInputProvider: fakeRuntimeInputProvider{input: RuntimeInput{
				AffectedPods:       1,
				ClusterPods:        100,
				TotalWorkloads:     10,
				UnhealthyWorkloads: 1,
			}},
			Now: func() time.Time { return now },
		}
		if err := o.Process(context.Background(), req); err == nil {
			t.Fatalf("expected maintenance window block")
		}
		if req.Status.BlockReasonCode != "gate_blocked" {
			t.Fatalf("expected gate_blocked reason code")
		}
	})

	t.Run("rate limit", func(t *testing.T) {
		req := newReq()
		req.Spec.Workload.Kind = "StatefulSet"
		req.Spec.RateLimit.MaxActions = 1
		req.Spec.RateLimit.WindowMinutes = 10
		now := time.Unix(1000, 0)
		o := &Orchestrator{
			Adapter:     fakeAdapter{supports: true},
			Snapshotter: &MemorySnapshotter{},
			RuntimeInputProvider: fakeRuntimeInputProvider{input: RuntimeInput{
				AffectedPods:       1,
				ClusterPods:        100,
				TotalWorkloads:     10,
				UnhealthyWorkloads: 1,
			}},
			Now: func() time.Time { return now },
		}
		o.actionHistory = map[string][]time.Time{req.Spec.Workload.Namespace + "/" + req.Spec.Workload.Name: []time.Time{now.Add(-time.Minute)}}
		if err := o.Process(context.Background(), req); err == nil {
			t.Fatalf("expected rate limit block")
		}
		if req.Status.BlockReasonCode != "gate_blocked" {
			t.Fatalf("expected gate_blocked reason code")
		}
	})

	t.Run("blast radius", func(t *testing.T) {
		req := newReq()
		req.Spec.Workload.Kind = "StatefulSet"
		req.Spec.BlastRadius.MaxPodPercentage = 10
		o := &Orchestrator{
			Adapter:     fakeAdapter{supports: true},
			Snapshotter: &MemorySnapshotter{},
			RuntimeInputProvider: fakeRuntimeInputProvider{input: RuntimeInput{
				AffectedPods:       30,
				ClusterPods:        100,
				TotalWorkloads:     10,
				UnhealthyWorkloads: 1,
			}},
		}
		if err := o.Process(context.Background(), req); err == nil {
			t.Fatalf("expected blast radius block")
		}
		if req.Status.BlockReasonCode != "gate_blocked" {
			t.Fatalf("expected gate_blocked reason code")
		}
	})
}

func TestOrchestratorObservabilityDegradedStillBlocksSafely(t *testing.T) {
	req := newReq()
	req.Spec.Workload.Kind = "StatefulSet"
	o := &Orchestrator{
		Adapter:     fakeAdapter{supports: true, totalWorkloads: 10, unhealthyWorkloads: 4},
		Snapshotter: &MemorySnapshotter{},
		EventSink:   nil,
		AuditSink:   nil,
	}
	if err := o.Process(context.Background(), req); err == nil {
		t.Fatalf("expected safe readonly block even when observability sinks are nil")
	}
	if req.Status.Phase != ksv1alpha1.PhaseBlocked {
		t.Fatalf("expected blocked phase")
	}
}
