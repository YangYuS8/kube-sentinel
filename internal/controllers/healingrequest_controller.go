package controllers

import (
	"context"
	"time"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ksv1alpha1 "github.com/yangyus8/kube-sentinel/api/v1alpha1"
	"github.com/yangyus8/kube-sentinel/internal/healing"
	"github.com/yangyus8/kube-sentinel/internal/notify"
	"github.com/yangyus8/kube-sentinel/internal/observability"
)

type HealingRequestReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	Recorder     record.EventRecorder
	Orchestrator *healing.Orchestrator
	EventSink    observability.EventSink
	Notifier     notify.TelegramNotifier
}

func (r *HealingRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var resource ksv1alpha1.HealingRequest
	if err := r.Get(ctx, req.NamespacedName, &resource); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	originalStatus := resource.Status
	if r.Orchestrator == nil {
		r.Orchestrator = &healing.Orchestrator{
			Adapter:          healing.NewDeploymentAdapter(r.Client),
			Snapshotter:      healing.NewKubernetesSnapshotter(r.Client),
			Metrics:          &observability.Metrics{},
			AuditSink:        &observability.MemoryAuditSink{},
			EventSink:        &observability.MemoryEventSink{},
			K8sEventRecorder: r.Recorder,
			Mode:             healing.RuntimeModeMinimal,
		}
	}
	result, err := r.Orchestrator.Process(ctx, &resource)
	statusChanged := !apiequality.Semantic.DeepEqual(originalStatus, resource.Status)
	if patchErr := r.patchStatus(ctx, req.NamespacedName, originalStatus, resource.Status); patchErr != nil {
		return ctrl.Result{}, patchErr
	}
	if statusChanged {
		r.notifyTelegram(ctx, &resource)
	}
	if err != nil {
		return ctrl.Result{RequeueAfter: result.RequeueAfter}, nil
	}
	return ctrl.Result{RequeueAfter: result.RequeueAfter}, nil
}

func (r *HealingRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorderFor("kube-sentinel")
	}
	if r.EventSink == nil && r.Orchestrator != nil {
		r.EventSink = r.Orchestrator.EventSink
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&ksv1alpha1.HealingRequest{}).
		Complete(r)
}

func (r *HealingRequestReconciler) patchStatus(ctx context.Context, key client.ObjectKey, original, desired ksv1alpha1.HealingRequestStatus) error {
	if apiequality.Semantic.DeepEqual(original, desired) {
		return nil
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var latest ksv1alpha1.HealingRequest
		if err := r.Get(ctx, key, &latest); err != nil {
			return client.IgnoreNotFound(err)
		}
		if apiequality.Semantic.DeepEqual(latest.Status, desired) {
			return nil
		}
		base := latest.DeepCopyObject().(*ksv1alpha1.HealingRequest)
		latest.Status = desired
		return r.Status().Patch(ctx, &latest, client.MergeFrom(base))
	})
}

func (r *HealingRequestReconciler) notifyTelegram(ctx context.Context, resource *ksv1alpha1.HealingRequest) {
	if r.Notifier == nil || resource == nil {
		return
	}
	if err := r.Notifier.Notify(ctx, resource); err != nil {
		if r.Recorder != nil {
			r.Recorder.Event(resource, "Warning", "TelegramNotificationFailed", err.Error())
		}
		if r.EventSink != nil {
			r.EventSink.Record(observability.RuntimeEvent{
				CorrelationKey: resource.Status.CorrelationKey,
				Namespace:      resource.Spec.Workload.Namespace,
				Name:           resource.Spec.Workload.Name,
				ResourceKind:   resource.Spec.Workload.Kind,
				Reason:         "TelegramNotificationFailed",
				Message:        err.Error(),
				Type:           "Warning",
				CreatedAt:      time.Now(),
			})
		}
		return
	}
	if r.Recorder != nil {
		r.Recorder.Event(resource, "Normal", "TelegramNotificationSent", "telegram incident card delivered")
	}
	if r.EventSink != nil {
		r.EventSink.Record(observability.RuntimeEvent{
			CorrelationKey: resource.Status.CorrelationKey,
			Namespace:      resource.Spec.Workload.Namespace,
			Name:           resource.Spec.Workload.Name,
			ResourceKind:   resource.Spec.Workload.Kind,
			Reason:         "TelegramNotificationSent",
			Message:        "telegram incident card delivered",
			Type:           "Normal",
			CreatedAt:      time.Now(),
		})
	}
}
