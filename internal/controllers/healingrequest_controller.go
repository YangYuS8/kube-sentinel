package controllers

import (
	"context"
	"fmt"
	"sync"
	"time"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ksv1alpha1 "github.com/yangyus8/kube-sentinel/api/v1alpha1"
	"github.com/yangyus8/kube-sentinel/internal/agent"
	"github.com/yangyus8/kube-sentinel/internal/healing"
	"github.com/yangyus8/kube-sentinel/internal/notify"
	"github.com/yangyus8/kube-sentinel/internal/observability"
)

const defaultTelegramFailureSuppressionWindow = 2 * time.Minute

type telegramNotificationRecord struct {
	Outcome     string
	OncallState agent.OncallState
	RecordedAt  time.Time
}

type HealingRequestReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	Recorder     record.EventRecorder
	Orchestrator *healing.Orchestrator
	EventSink    observability.EventSink
	Notifier     notify.TelegramNotifier
	Now          func() time.Time

	telegramMu      sync.Mutex
	telegramRecords map[string]telegramNotificationRecord
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
	if r.Now == nil {
		r.Now = time.Now
	}
	if r.telegramRecords == nil {
		r.telegramRecords = map[string]telegramNotificationRecord{}
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
	if r.Now == nil {
		r.Now = time.Now
	}
	if r.telegramRecords == nil {
		r.telegramRecords = map[string]telegramNotificationRecord{}
	}
	oncallState := agent.TranslateOncallState(resource)
	notifyKey := r.telegramNotificationKey(resource, oncallState)
	if suppress, reason := r.shouldSuppressTelegramNotification(notifyKey, oncallState); suppress {
		r.recordTelegramRuntimeEvent(resource, "TelegramNotificationSuppressed", reason, "Normal")
		return
	}
	if err := r.Notifier.Notify(ctx, resource); err != nil {
		r.rememberTelegramNotification(notifyKey, oncallState, "failed")
		if r.Recorder != nil {
			r.Recorder.Event(resource, "Warning", "TelegramNotificationFailed", err.Error())
		}
		r.recordTelegramRuntimeEvent(resource, "TelegramNotificationFailed", err.Error(), "Warning")
		return
	}
	r.rememberTelegramNotification(notifyKey, oncallState, "sent")
	if r.Recorder != nil {
		r.Recorder.Event(resource, "Normal", "TelegramNotificationSent", "telegram incident card delivered")
	}
	r.recordTelegramRuntimeEvent(resource, "TelegramNotificationSent", fmt.Sprintf("telegram incident card delivered (%s)", oncallState), "Normal")
}

func (r *HealingRequestReconciler) telegramNotificationKey(resource *ksv1alpha1.HealingRequest, oncallState agent.OncallState) string {
	correlation := resource.Status.CorrelationKey
	if correlation == "" {
		correlation = resource.Namespace + "/" + resource.Name
	}
	return correlation + "|" + string(oncallState)
}

func (r *HealingRequestReconciler) shouldSuppressTelegramNotification(key string, oncallState agent.OncallState) (bool, string) {
	r.telegramMu.Lock()
	defer r.telegramMu.Unlock()
	record, ok := r.telegramRecords[key]
	if !ok {
		return false, ""
	}
	now := r.Now()
	if record.Outcome == "sent" {
		return true, fmt.Sprintf("suppressed duplicate telegram notification for oncall state %s", oncallState)
	}
	if record.Outcome == "failed" && now.Sub(record.RecordedAt) < defaultTelegramFailureSuppressionWindow {
		return true, fmt.Sprintf("suppressed repeated telegram failure for oncall state %s", oncallState)
	}
	return false, ""
}

func (r *HealingRequestReconciler) rememberTelegramNotification(key string, oncallState agent.OncallState, outcome string) {
	r.telegramMu.Lock()
	defer r.telegramMu.Unlock()
	r.telegramRecords[key] = telegramNotificationRecord{Outcome: outcome, OncallState: oncallState, RecordedAt: r.Now()}
}

func (r *HealingRequestReconciler) recordTelegramRuntimeEvent(resource *ksv1alpha1.HealingRequest, reason, message, eventType string) {
	if r.EventSink != nil {
		r.EventSink.Record(observability.RuntimeEvent{
			CorrelationKey: resource.Status.CorrelationKey,
			Namespace:      resource.Spec.Workload.Namespace,
			Name:           resource.Spec.Workload.Name,
			ResourceKind:   resource.Spec.Workload.Kind,
			Reason:         reason,
			Message:        message,
			Type:           eventType,
			CreatedAt:      r.Now(),
		})
	}
}
