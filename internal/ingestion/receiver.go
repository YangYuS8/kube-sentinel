package ingestion

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ksv1alpha1 "github.com/yangyus8/kube-sentinel/api/v1alpha1"
	"github.com/yangyus8/kube-sentinel/internal/observability"
)

const (
	labelWorkloadKind = "workload_kind"
	labelNamespace    = "namespace"
	labelName         = "name"
)

type Receiver struct {
	Client    client.Client
	Dedupe    DedupeStore
	AuditSink observability.AuditSink
	Now       func() time.Time
}

func (r *Receiver) HandleWebhook(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	payload := AlertmanagerPayload{}
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if len(payload.Alerts) == 0 {
		http.Error(w, "no alerts in payload", http.StatusBadRequest)
		return
	}
	if r.Now == nil {
		r.Now = time.Now
	}
	if r.Dedupe == nil {
		r.Dedupe = NewMemoryDedupeStore()
	}

	for _, alert := range payload.Alerts {
		event, err := mapAlert(alert)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !supportsWorkloadKind(event.WorkloadKind) {
			r.writeAudit(event, "read-only-reject", "unsupported workload kind")
			continue
		}
		window, err := r.resolveIdempotencyWindow(req.Context(), event)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to resolve idempotency window: %v", err), http.StatusInternalServerError)
			return
		}
		if r.Dedupe.Seen(event.CorrelationKey, r.Now(), window) {
			r.writeAudit(event, "dedupe-skip", "duplicate event within idempotency window")
			continue
		}
		if err := r.upsertHealingRequest(req.Context(), event); err != nil {
			http.Error(w, fmt.Sprintf("failed to create/update HealingRequest: %v", err), http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte("accepted"))
}

func mapAlert(alert Alert) (Event, error) {
	kind := strings.TrimSpace(alert.Labels[labelWorkloadKind])
	ns := strings.TrimSpace(alert.Labels[labelNamespace])
	name := strings.TrimSpace(alert.Labels[labelName])
	if kind == "" || ns == "" || name == "" {
		return Event{}, fmt.Errorf("missing required labels: workload_kind, namespace, name")
	}
	fingerprint := strings.TrimSpace(alert.Fingerprint)
	if fingerprint == "" {
		fingerprint = fmt.Sprintf("%s/%s/%s", kind, ns, name)
	}
	reason := strings.TrimSpace(alert.Annotations["summary"])
	if reason == "" {
		reason = "alertmanager webhook"
	}
	return Event{
		Fingerprint:    fingerprint,
		CorrelationKey: fingerprint,
		WorkloadKind:   kind,
		Namespace:      ns,
		Name:           name,
		Reason:         reason,
		AlertStatus:    strings.TrimSpace(alert.Status),
		AlertCategory:  strings.TrimSpace(alert.Labels["alertname"]),
		AlertSeverity:  strings.TrimSpace(alert.Labels["severity"]),
	}, nil
}

func (r *Receiver) upsertHealingRequest(ctx context.Context, event Event) error {
	name := fmt.Sprintf("hr-%s", event.Name)
	key := types.NamespacedName{Name: name, Namespace: event.Namespace}
	var obj ksv1alpha1.HealingRequest
	err := r.Client.Get(ctx, key, &obj)
	if err == nil {
		obj.Annotations = ensureMap(obj.Annotations)
		obj.Annotations["kube-sentinel.io/correlation-key"] = event.CorrelationKey
		obj.Annotations["kube-sentinel.io/alert-status"] = event.AlertStatus
		obj.Annotations["kube-sentinel.io/alert-category"] = event.AlertCategory
		obj.Annotations["kube-sentinel.io/alert-severity"] = event.AlertSeverity
		obj.Annotations["kube-sentinel.io/workload-capability"] = workloadCapabilityForKind(event.WorkloadKind)
		obj.Status.CorrelationKey = event.CorrelationKey
		obj.Status.LastEventReason = event.Reason
		obj.Status.WorkloadCapability = workloadCapabilityForKind(event.WorkloadKind)
		return r.Client.Update(ctx, &obj)
	}

	create := ksv1alpha1.HealingRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: event.Namespace,
			Annotations: map[string]string{
				"kube-sentinel.io/correlation-key":     event.CorrelationKey,
				"kube-sentinel.io/alert-status":        event.AlertStatus,
				"kube-sentinel.io/alert-category":      event.AlertCategory,
				"kube-sentinel.io/alert-severity":      event.AlertSeverity,
				"kube-sentinel.io/workload-capability": workloadCapabilityForKind(event.WorkloadKind),
			},
		},
		Spec: ksv1alpha1.HealingRequestSpec{
			Workload: ksv1alpha1.WorkloadRef{Kind: event.WorkloadKind, Namespace: event.Namespace, Name: event.Name},
		},
	}
	create.ApplyDefaults()
	if err := create.Validate(); err != nil {
		return err
	}
	return r.Client.Create(ctx, &create)
}

func (r *Receiver) resolveIdempotencyWindow(ctx context.Context, event Event) (time.Duration, error) {
	if r.Client == nil {
		return 0, fmt.Errorf("kubernetes client is required")
	}
	name := fmt.Sprintf("hr-%s", event.Name)
	key := types.NamespacedName{Name: name, Namespace: event.Namespace}
	obj := ksv1alpha1.HealingRequest{}
	err := r.Client.Get(ctx, key, &obj)
	if err != nil && !apierrors.IsNotFound(err) {
		return 0, err
	}
	if apierrors.IsNotFound(err) {
		obj = ksv1alpha1.HealingRequest{Spec: ksv1alpha1.HealingRequestSpec{Workload: ksv1alpha1.WorkloadRef{Kind: event.WorkloadKind, Namespace: event.Namespace, Name: event.Name}}}
	}
	obj.ApplyDefaults()
	if obj.Spec.IdempotencyWindowMinutes < 1 {
		return 0, fmt.Errorf("idempotencyWindowMinutes must be >= 1")
	}
	return time.Duration(obj.Spec.IdempotencyWindowMinutes) * time.Minute, nil
}

func ensureMap(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

func (r *Receiver) writeAudit(event Event, result, detail string) {
	if r.AuditSink == nil {
		return
	}
	r.AuditSink.Write(observability.AuditEvent{
		ID:           event.CorrelationKey,
		Trigger:      "alertmanager",
		Target:       event.Namespace + "/" + event.Name,
		WorkloadKind: event.WorkloadKind,
		BeforeState:  "event-received",
		AfterState:   detail,
		Result:       result,
		CreatedAt:    r.Now(),
	})
}

func supportsWorkloadKind(kind string) bool {
	return kind == "Deployment" || kind == "StatefulSet"
}

func workloadCapabilityForKind(kind string) string {
	if kind == "StatefulSet" {
		return "read-only"
	}
	return "writable"
}
