package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ksv1alpha1 "github.com/yangyus8/kube-sentinel/api/v1alpha1"
)

func testRequest() *ksv1alpha1.HealingRequest {
	return &ksv1alpha1.HealingRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hr-api",
			Namespace: "default",
			Annotations: map[string]string{
				"kube-sentinel.io/alert-category": "CrashLoopBackOff",
			},
		},
		Spec:   ksv1alpha1.HealingRequestSpec{Workload: ksv1alpha1.WorkloadRef{Kind: "Deployment", Namespace: "default", Name: "api"}},
		Status: ksv1alpha1.HealingRequestStatus{Phase: ksv1alpha1.PhaseBlocked, LastAction: "manual-intervention", BlockReasonCode: "snapshot_failed", NextRecommendation: "inspect rollout status", RecommendationType: "investigate", CorrelationKey: "default/hr-api"},
	}
}

func TestNewTelegramNotifierRequiresConfig(t *testing.T) {
	if notifier := NewTelegramNotifier(TelegramConfig{}); notifier != nil {
		t.Fatalf("expected nil notifier without config")
	}
}

func TestTelegramNotifierSendsTwoMessages(t *testing.T) {
	var requests []sendMessageRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()
		var payload sendMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		requests = append(requests, payload)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	notifier := NewTelegramNotifier(TelegramConfig{BotToken: "token", ChatID: "chat", BaseURL: server.URL})
	if err := notifier.Notify(context.Background(), testRequest()); err != nil {
		t.Fatalf("notify failed: %v", err)
	}
	if len(requests) != 2 {
		t.Fatalf("expected 2 telegram requests, got %d", len(requests))
	}
	if requests[0].ChatID != "chat" {
		t.Fatalf("expected chat id preserved")
	}
	if requests[0].Text == "" || requests[1].Text == "" {
		t.Fatalf("expected non-empty messages")
	}
}

func TestTelegramNotifierReturnsErrorOnHTTPFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()
	notifier := NewTelegramNotifier(TelegramConfig{BotToken: "token", ChatID: "chat", BaseURL: server.URL})
	if err := notifier.Notify(context.Background(), testRequest()); err == nil {
		t.Fatalf("expected notify error")
	}
}
