package controllers

import (
	"context"
	"errors"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	ksv1alpha1 "github.com/yangyus8/kube-sentinel/api/v1alpha1"
	"github.com/yangyus8/kube-sentinel/internal/notify"
	"github.com/yangyus8/kube-sentinel/internal/observability"
)

type stubNotifier struct{ err error }

func (s stubNotifier) Notify(context.Context, *ksv1alpha1.HealingRequest) error { return s.err }

type conflictOnceClient struct {
	client.Client
	patchCalls int
}

func (c *conflictOnceClient) Status() client.SubResourceWriter {
	return &conflictOnceStatusWriter{parent: c, delegate: c.Client.Status()}
}

type conflictOnceStatusWriter struct {
	parent   *conflictOnceClient
	delegate client.SubResourceWriter
}

func (w *conflictOnceStatusWriter) Create(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	return w.delegate.Create(ctx, obj, subResource, opts...)
}

func (w *conflictOnceStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	return w.delegate.Update(ctx, obj, opts...)
}

func (w *conflictOnceStatusWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	w.parent.patchCalls++
	if w.parent.patchCalls == 1 {
		return apierrors.NewConflict(
			schema.GroupResource{Group: ksv1alpha1.GroupVersion.Group, Resource: "healingrequests"},
			obj.GetName(),
			errors.New("simulated conflict"),
		)
	}
	return w.delegate.Patch(ctx, obj, patch, opts...)
}

func TestPatchStatusRetriesOnConflict(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := ksv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme: %v", err)
	}

	existing := &ksv1alpha1.HealingRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "hr-app", Namespace: "default"},
		Status: ksv1alpha1.HealingRequestStatus{
			Phase: ksv1alpha1.PhasePending,
		},
	}

	baseClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(existing).
		WithObjects(existing).
		Build()

	wrappedClient := &conflictOnceClient{Client: baseClient}
	reconciler := &HealingRequestReconciler{Client: wrappedClient}

	desired := existing.Status
	desired.Phase = ksv1alpha1.PhaseCompleted
	desired.LastAction = "deployment-l1-rollout-restart"

	if err := reconciler.patchStatus(context.Background(), client.ObjectKeyFromObject(existing), existing.Status, desired); err != nil {
		t.Fatalf("patch status: %v", err)
	}
	if wrappedClient.patchCalls < 2 {
		t.Fatalf("expected conflict retry, got %d patch calls", wrappedClient.patchCalls)
	}

	var stored ksv1alpha1.HealingRequest
	if err := wrappedClient.Get(context.Background(), client.ObjectKeyFromObject(existing), &stored); err != nil {
		t.Fatalf("get stored request: %v", err)
	}
	if stored.Status.Phase != ksv1alpha1.PhaseCompleted {
		t.Fatalf("expected completed phase, got %s", stored.Status.Phase)
	}
	if stored.Status.LastAction != "deployment-l1-rollout-restart" {
		t.Fatalf("expected persisted last action, got %s", stored.Status.LastAction)
	}
}

func TestNotifyTelegramRecordsSuccessEvent(t *testing.T) {
	reconciler := &HealingRequestReconciler{Notifier: stubNotifier{}, EventSink: &observability.MemoryEventSink{}}
	resource := &ksv1alpha1.HealingRequest{
		Spec:   ksv1alpha1.HealingRequestSpec{Workload: ksv1alpha1.WorkloadRef{Kind: "Deployment", Namespace: "default", Name: "api"}},
		Status: ksv1alpha1.HealingRequestStatus{CorrelationKey: "default/hr-api"},
	}
	reconciler.notifyTelegram(context.Background(), resource)
	events := reconciler.EventSink.(*observability.MemoryEventSink).Events
	if len(events) != 1 {
		t.Fatalf("expected one runtime event, got %d", len(events))
	}
	if events[0].Reason != "TelegramNotificationSent" {
		t.Fatalf("expected TelegramNotificationSent, got %s", events[0].Reason)
	}
}

func TestNotifyTelegramRecordsFailureWithoutReturning(t *testing.T) {
	reconciler := &HealingRequestReconciler{Notifier: stubNotifier{err: errors.New("boom")}, EventSink: &observability.MemoryEventSink{}}
	resource := &ksv1alpha1.HealingRequest{
		Spec:   ksv1alpha1.HealingRequestSpec{Workload: ksv1alpha1.WorkloadRef{Kind: "Deployment", Namespace: "default", Name: "api"}},
		Status: ksv1alpha1.HealingRequestStatus{CorrelationKey: "default/hr-api"},
	}
	reconciler.notifyTelegram(context.Background(), resource)
	events := reconciler.EventSink.(*observability.MemoryEventSink).Events
	if len(events) != 1 {
		t.Fatalf("expected one runtime event, got %d", len(events))
	}
	if events[0].Reason != "TelegramNotificationFailed" {
		t.Fatalf("expected TelegramNotificationFailed, got %s", events[0].Reason)
	}
}

var _ notify.TelegramNotifier = stubNotifier{}
