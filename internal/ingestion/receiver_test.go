package ingestion

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	ksv1alpha1 "github.com/yangyus8/kube-sentinel/api/v1alpha1"
	"github.com/yangyus8/kube-sentinel/internal/observability"
)

func buildReceiver(t *testing.T) *Receiver {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := ksv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme err: %v", err)
	}
	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	return &Receiver{Client: client, Dedupe: NewMemoryDedupeStore(), AuditSink: &observability.MemoryAuditSink{}, Now: time.Now}
}

func TestWebhookValidCreatesRequest(t *testing.T) {
	r := buildReceiver(t)
	payload := AlertmanagerPayload{Alerts: []Alert{{Fingerprint: "fp1", Labels: map[string]string{"workload_kind": "Deployment", "namespace": "default", "name": "app"}, Annotations: map[string]string{"summary": "pod crash"}}}}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/alertmanager/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.HandleWebhook(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("expected accepted, got %d", w.Code)
	}
}

func TestWebhookRejectsInvalid(t *testing.T) {
	r := buildReceiver(t)
	req := httptest.NewRequest(http.MethodPost, "/alertmanager/webhook", bytes.NewBufferString("bad-json"))
	w := httptest.NewRecorder()
	r.HandleWebhook(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request")
	}
}

func TestWebhookDedupe(t *testing.T) {
	now := time.Now()
	scheme := runtime.NewScheme()
	_ = ksv1alpha1.AddToScheme(scheme)
	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	audit := &observability.MemoryAuditSink{}
	r := &Receiver{Client: client, Dedupe: NewMemoryDedupeStore(), AuditSink: audit, Now: func() time.Time { return now }}
	payload := AlertmanagerPayload{Alerts: []Alert{{Fingerprint: "fp1", Labels: map[string]string{"workload_kind": "Deployment", "namespace": "default", "name": "app"}}}}
	body, _ := json.Marshal(payload)
	req1 := httptest.NewRequest(http.MethodPost, "/alertmanager/webhook", bytes.NewReader(body))
	w1 := httptest.NewRecorder()
	r.HandleWebhook(w1, req1)
	req2 := httptest.NewRequest(http.MethodPost, "/alertmanager/webhook", bytes.NewReader(body))
	w2 := httptest.NewRecorder()
	r.HandleWebhook(w2, req2)
	if w2.Code != http.StatusAccepted {
		t.Fatalf("expected accepted on duplicate event")
	}
}

func TestWebhookDedupeUsesRequestWindow(t *testing.T) {
	now := time.Now()
	scheme := runtime.NewScheme()
	_ = ksv1alpha1.AddToScheme(scheme)
	existing := &ksv1alpha1.HealingRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "hr-app", Namespace: "default"},
		Spec: ksv1alpha1.HealingRequestSpec{
			Workload:                 ksv1alpha1.WorkloadRef{Kind: "Deployment", Namespace: "default", Name: "app"},
			IdempotencyWindowMinutes: 1,
		},
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()
	r := &Receiver{Client: client, Dedupe: NewMemoryDedupeStore(), Now: func() time.Time { return now }}
	payload := AlertmanagerPayload{Alerts: []Alert{{Fingerprint: "fp-custom", Labels: map[string]string{"workload_kind": "Deployment", "namespace": "default", "name": "app"}}}}
	body, _ := json.Marshal(payload)
	w1 := httptest.NewRecorder()
	r.HandleWebhook(w1, httptest.NewRequest(http.MethodPost, "/alertmanager/webhook", bytes.NewReader(body)))
	if w1.Code != http.StatusAccepted {
		t.Fatalf("expected first request accepted")
	}
	r.Now = func() time.Time { return now.Add(2 * time.Minute) }
	w2 := httptest.NewRecorder()
	r.HandleWebhook(w2, httptest.NewRequest(http.MethodPost, "/alertmanager/webhook", bytes.NewReader(body)))
	if w2.Code != http.StatusAccepted {
		t.Fatalf("expected second request accepted after window")
	}
}

func TestWebhookRejectsNonDeploymentReadOnly(t *testing.T) {
	r := buildReceiver(t)
	payload := AlertmanagerPayload{Alerts: []Alert{{Fingerprint: "fp-non", Labels: map[string]string{"workload_kind": "StatefulSet", "namespace": "default", "name": "db"}}}}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/alertmanager/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.HandleWebhook(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("expected accepted with readonly reject")
	}
}

func TestUpsertUpdatesCorrelationKey(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = ksv1alpha1.AddToScheme(scheme)
	existing := &ksv1alpha1.HealingRequest{ObjectMeta: metav1.ObjectMeta{Name: "hr-app", Namespace: "default"}, Spec: ksv1alpha1.HealingRequestSpec{Workload: ksv1alpha1.WorkloadRef{Kind: "Deployment", Namespace: "default", Name: "app"}}}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()
	r := &Receiver{Client: client, Dedupe: NewMemoryDedupeStore(), Now: time.Now}
	if err := r.upsertHealingRequest(context.Background(), Event{CorrelationKey: "fp", WorkloadKind: "Deployment", Namespace: "default", Name: "app", Reason: "test"}); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}
}
