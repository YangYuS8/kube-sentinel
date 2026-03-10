package agent

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ksv1alpha1 "github.com/yangyus8/kube-sentinel/api/v1alpha1"
	"github.com/yangyus8/kube-sentinel/internal/observability"
)

func newRequest() *ksv1alpha1.HealingRequest {
	return &ksv1alpha1.HealingRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "hr-api",
			Namespace:         "default",
			CreationTimestamp: metav1.Now(),
			Annotations: map[string]string{
				"kube-sentinel.io/alert-category": "CrashLoopBackOff",
				"kube-sentinel.io/alert-severity": "critical",
			},
		},
		Spec: ksv1alpha1.HealingRequestSpec{Workload: ksv1alpha1.WorkloadRef{Kind: "Deployment", Namespace: "default", Name: "api"}},
		Status: ksv1alpha1.HealingRequestStatus{
			Phase:              ksv1alpha1.PhaseBlocked,
			LastAction:         "manual-intervention",
			BlockReasonCode:    "snapshot_failed",
			LastGateDecision:   "outcome=block reason_code=snapshot_failed stage=pre-l1",
			NextRecommendation: "fix snapshot creation or continue with manual intervention",
			RecommendationType: "investigate",
			CorrelationKey:     "default/hr-api",
		},
	}
}

func TestStatusFieldTiers(t *testing.T) {
	if StatusFieldTiers["phase"] != InputTierCore {
		t.Fatalf("expected phase to be core")
	}
	if StatusFieldTiers["snapshotFailureReason"] != InputTierEvidence {
		t.Fatalf("expected snapshotFailureReason to be evidence")
	}
	if StatusFieldTiers["deploymentL2Decision"] != InputTierLegacy {
		t.Fatalf("expected deploymentL2Decision to be legacy")
	}
	if StatusFieldTiers["statefulSetL2Result"] != InputTierLegacy {
		t.Fatalf("expected statefulSetL2Result to be legacy")
	}
}

func TestBuildReportBlockedIncident(t *testing.T) {
	req := newRequest()
	report := BuildReport(req, Evidence{})
	if report.CurrentFocus != FocusSafetyBlocked {
		t.Fatalf("expected safety-blocked focus, got %s", report.CurrentFocus)
	}
	if report.Notification.Kind != NotificationBlocked {
		t.Fatalf("expected blocked notification, got %s", report.Notification.Kind)
	}
	if len(report.NextSteps) == 0 {
		t.Fatalf("expected next steps")
	}
	if !strings.Contains(report.Notification.LongMessage, "where next") {
		t.Fatalf("expected long notification to contain entry points")
	}
	if !strings.Contains(report.Handoff, "correlation: default/hr-api") {
		t.Fatalf("expected handoff to include correlation key")
	}
}

func TestBuildReportAutoTriedIncident(t *testing.T) {
	req := newRequest()
	req.Status.Phase = ksv1alpha1.PhaseCompleted
	req.Status.LastAction = "deployment-l1-rollout-restart"
	req.Status.BlockReasonCode = ""
	req.Status.LastError = ""
	req.Status.NextRecommendation = "continue observing post-l1 stability"
	report := BuildReport(req, Evidence{})
	if report.Notification.Kind != NotificationAutoTried {
		t.Fatalf("expected auto-tried notification, got %s", report.Notification.Kind)
	}
	if report.CurrentFocus != FocusStartupFailure {
		t.Fatalf("expected startup-failure focus, got %s", report.CurrentFocus)
	}
	if !strings.Contains(report.Notification.ShortMessage, "auto-tried") {
		t.Fatalf("expected short notification to mention auto-tried")
	}
}

func TestBuildReportRecoveredIncident(t *testing.T) {
	req := newRequest()
	req.Status.Phase = ksv1alpha1.PhaseSuppressed
	req.Status.LastAction = "suppressed"
	req.Annotations["kube-sentinel.io/alert-status"] = "resolved"
	report := BuildReport(req, Evidence{})
	if report.CurrentFocus != FocusTransientRecovered {
		t.Fatalf("expected transient focus, got %s", report.CurrentFocus)
	}
	if report.Notification.Kind != NotificationRecovered {
		t.Fatalf("expected recovered notification, got %s", report.Notification.Kind)
	}
}

func TestBuildReportConfigDependencyFocusFromEvidence(t *testing.T) {
	req := newRequest()
	req.Status.BlockReasonCode = ""
	req.Status.LastError = "missing secret for startup"
	report := BuildReport(req, Evidence{LatestRuntimeEvent: &observability.RuntimeEvent{Reason: "ConfigError", Message: "secret missing"}})
	if report.CurrentFocus != FocusConfigOrDependency {
		t.Fatalf("expected config-or-dependency focus, got %s", report.CurrentFocus)
	}
}

func TestBuildReportInsufficientEvidence(t *testing.T) {
	req := &ksv1alpha1.HealingRequest{}
	report := BuildReport(req, Evidence{})
	if report.CurrentFocus != FocusInsufficientEvidence {
		t.Fatalf("expected insufficient-evidence focus, got %s", report.CurrentFocus)
	}
}
