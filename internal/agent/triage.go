package agent

import (
	"fmt"
	"strings"
	"time"

	ksv1alpha1 "github.com/yangyus8/kube-sentinel/api/v1alpha1"
	"github.com/yangyus8/kube-sentinel/internal/observability"
)

type InputTier string

const (
	InputTierCore     InputTier = "core"
	InputTierEvidence InputTier = "evidence"
	InputTierLegacy   InputTier = "legacy"
)

type Focus string

const (
	FocusStartupFailure       Focus = "startup-failure"
	FocusConfigOrDependency   Focus = "config-or-dependency"
	FocusSafetyBlocked        Focus = "safety-blocked"
	FocusTransientRecovered   Focus = "transient-or-recovered"
	FocusManualFollowUp       Focus = "manual-follow-up"
	FocusInsufficientEvidence Focus = "insufficient-evidence"
)

type NotificationKind string

const (
	NotificationAutoTried NotificationKind = "auto-tried"
	NotificationBlocked   NotificationKind = "blocked"
	NotificationRecovered NotificationKind = "recovered"
)

var StatusFieldTiers = map[string]InputTier{
	"phase":                    InputTierCore,
	"workloadCapability":       InputTierEvidence,
	"incidentSummary":          InputTierCore,
	"recommendationType":       InputTierCore,
	"handoffNote":              InputTierCore,
	"nextRecommendation":       InputTierCore,
	"blockReasonCode":          InputTierCore,
	"lastAction":               InputTierCore,
	"lastError":                InputTierCore,
	"lastGateDecision":         InputTierCore,
	"gateOutcome":              InputTierEvidence,
	"gateReasonCode":           InputTierEvidence,
	"gateEvidenceComplete":     InputTierEvidence,
	"lastEvidenceStatus":       InputTierEvidence,
	"lastEventReason":          InputTierEvidence,
	"lastSnapshotId":           InputTierEvidence,
	"snapshotRestoreResult":    InputTierEvidence,
	"snapshotFailureReason":    InputTierEvidence,
	"snapshotActiveCount":      InputTierEvidence,
	"pendingSince":             InputTierEvidence,
	"suppressedAt":             InputTierEvidence,
	"stableSampleCount":        InputTierEvidence,
	"shadowAction":             InputTierEvidence,
	"correlationKey":           InputTierCore,
	"auditRef":                 InputTierEvidence,
	"observedGeneration":       InputTierEvidence,
	"circuitBreaker":           InputTierEvidence,
	"conditions":               InputTierEvidence,
	"namespaceBlockRate":       InputTierLegacy,
	"emergencyAttempts":        InputTierLegacy,
	"lastHealthyRevision":      InputTierLegacy,
	"deploymentL2Candidate":    InputTierLegacy,
	"deploymentL2Decision":     InputTierLegacy,
	"deploymentL2Result":       InputTierLegacy,
	"statefulSetAuthorization": InputTierLegacy,
	"statefulSetFreezeState":   InputTierLegacy,
	"statefulSetFreezeUntil":   InputTierLegacy,
	"statefulSetFailureReason": InputTierLegacy,
	"statefulSetL2Candidate":   InputTierLegacy,
	"statefulSetL2Decision":    InputTierLegacy,
	"statefulSetL2Result":      InputTierLegacy,
}

type Evidence struct {
	LatestAuditEvent   *observability.AuditEvent
	LatestRuntimeEvent *observability.RuntimeEvent
	Metrics            observability.TriageMetricsSnapshot
}

type Report struct {
	WhatHappened   []string
	WhatRuntimeDid []string
	CurrentFocus   Focus
	FocusDetail    string
	NextSteps      []string
	Handoff        string
	Summary        string
	Notification   Notification
}

type Notification struct {
	Kind         NotificationKind
	ShortMessage string
	LongMessage  string
}

func BuildReport(req *ksv1alpha1.HealingRequest, evidence Evidence) Report {
	report := Report{}
	if req == nil {
		report.CurrentFocus = FocusInsufficientEvidence
		report.FocusDetail = "incident object is unavailable"
		report.NextSteps = []string{"inspect runtime state source before continuing"}
		report.Handoff = "incident object unavailable; gather runtime context before handoff"
		report.Summary = "incident unavailable; evidence is insufficient"
		report.Notification = Notification{Kind: NotificationBlocked, ShortMessage: "[WARN] incident unavailable", LongMessage: "[WARN] Sentinel cannot read the incident object; gather runtime context before continuing."}
		return report
	}

	trigger := strings.TrimSpace(req.Annotations["kube-sentinel.io/alert-category"])
	if trigger == "" {
		trigger = strings.TrimSpace(req.Status.LastEventReason)
	}
	if trigger == "" {
		trigger = "unknown-trigger"
	}
	workload := fmt.Sprintf("%s/%s (%s)", req.Spec.Workload.Namespace, req.Spec.Workload.Name, req.Spec.Workload.Kind)
	report.WhatHappened = []string{
		fmt.Sprintf("workload: %s", workload),
		fmt.Sprintf("trigger: %s", trigger),
		fmt.Sprintf("phase: %s", req.Status.Phase),
	}

	report.WhatRuntimeDid = whatRuntimeDid(req)
	report.CurrentFocus, report.FocusDetail = classifyFocus(req, evidence, trigger)
	report.NextSteps = nextStepsFor(req, report.CurrentFocus)
	report.Handoff = buildHandoff(req, report.CurrentFocus)
	report.Summary = buildSummary(req, trigger, report.CurrentFocus)
	report.Notification = buildTelegramNotification(req, trigger, report)
	return report
}

func whatRuntimeDid(req *ksv1alpha1.HealingRequest) []string {
	if req == nil {
		return []string{"runtime action unavailable"}
	}
	phase := req.Status.Phase
	action := strings.TrimSpace(req.Status.LastAction)
	if action == "" || action == "manual-intervention" {
		action = "no automatic write action executed"
	}
	lines := []string{fmt.Sprintf("runtime action: %s", action)}
	switch phase {
	case ksv1alpha1.PhaseCompleted:
		lines = append(lines, "runtime result: completed current minimal action path")
	case ksv1alpha1.PhaseBlocked, ksv1alpha1.PhaseL3:
		reason := strings.TrimSpace(req.Status.BlockReasonCode)
		if reason == "" {
			reason = strings.TrimSpace(req.Status.LastError)
		}
		if reason == "" {
			reason = "manual follow-up required"
		}
		lines = append(lines, fmt.Sprintf("runtime result: blocked because %s", reason))
	case ksv1alpha1.PhaseSuppressed:
		lines = append(lines, "runtime result: no write action required after observation")
	case ksv1alpha1.PhasePendingVerify:
		lines = append(lines, "runtime result: observing before taking further action")
	default:
		lines = append(lines, fmt.Sprintf("runtime result: %s", phase))
	}
	return lines
}

func classifyFocus(req *ksv1alpha1.HealingRequest, evidence Evidence, trigger string) (Focus, string) {
	if req == nil || req.Spec.Workload.Name == "" || req.Status.Phase == "" {
		return FocusInsufficientEvidence, "core incident fields are incomplete"
	}
	if req.Status.Phase == ksv1alpha1.PhaseSuppressed || strings.EqualFold(strings.TrimSpace(req.Annotations["kube-sentinel.io/alert-status"]), "resolved") {
		return FocusTransientRecovered, "the alert recovered during the observation window"
	}
	if isSafetyBlocked(req) {
		return FocusSafetyBlocked, "runtime stopped because the current safety boundary does not justify another automatic action"
	}
	if suggestsConfigOrDependency(req, evidence) {
		return FocusConfigOrDependency, "recent evidence points to configuration, dependency, or missing prerequisite issues"
	}
	if suggestsStartupFailure(trigger, evidence) {
		return FocusStartupFailure, "the incident looks closer to a startup failure than to a broad safety or governance issue"
	}
	if req.Status.Phase == ksv1alpha1.PhaseBlocked || req.Status.Phase == ksv1alpha1.PhaseL3 {
		return FocusManualFollowUp, "runtime has reached a manual follow-up boundary"
	}
	if req.Status.Phase == ksv1alpha1.PhaseCompleted || req.Status.Phase == ksv1alpha1.PhasePendingVerify {
		return FocusTransientRecovered, "the workload is currently in observation or has already completed the minimal runtime path"
	}
	return FocusInsufficientEvidence, "the available inputs are not sufficient to narrow the issue further"
}

func isSafetyBlocked(req *ksv1alpha1.HealingRequest) bool {
	reason := strings.TrimSpace(req.Status.BlockReasonCode)
	switch reason {
	case "read_only_mode", "gate_blocked", "snapshot_failed", "circuit_breaker_open", "out_of_scope_workload", "namespace_budget_blocked", "runtime_input_unavailable", "deployment_l1_idempotency_window":
		return true
	default:
		return false
	}
}

func suggestsConfigOrDependency(req *ksv1alpha1.HealingRequest, evidence Evidence) bool {
	joined := strings.ToLower(strings.Join([]string{
		req.Status.LastError,
		req.Status.BlockReasonCode,
		req.Status.LastEventReason,
		req.Status.SnapshotFailureReason,
		req.Status.ShadowAction,
		req.Status.LastEvidenceStatus,
		evidenceMessage(evidence),
	}, " "))
	for _, token := range []string{"config", "configmap", "secret", "dependency", "missing", "imagepullbackoff", "errimagepull", "connection refused"} {
		if strings.Contains(joined, token) {
			return true
		}
	}
	return false
}

func suggestsStartupFailure(trigger string, evidence Evidence) bool {
	trigger = strings.ToLower(strings.TrimSpace(trigger))
	if strings.Contains(trigger, "crashloop") || strings.Contains(trigger, "oom") || strings.Contains(trigger, "probefailure") {
		return true
	}
	reason := strings.ToLower(evidenceMessage(evidence))
	return strings.Contains(reason, "crashloop") || strings.Contains(reason, "back-off") || strings.Contains(reason, "failed")
}

func evidenceMessage(e Evidence) string {
	parts := []string{}
	if e.LatestRuntimeEvent != nil {
		parts = append(parts, e.LatestRuntimeEvent.Reason, e.LatestRuntimeEvent.Message)
	}
	if e.LatestAuditEvent != nil {
		parts = append(parts, e.LatestAuditEvent.AfterState, e.LatestAuditEvent.Recommendation)
	}
	return strings.Join(parts, " ")
}

func nextStepsFor(req *ksv1alpha1.HealingRequest, focus Focus) []string {
	steps := []string{}
	if rec := strings.TrimSpace(req.Status.NextRecommendation); rec != "" {
		steps = append(steps, rec)
	}
	appendIfMissing := func(step string) {
		for _, existing := range steps {
			if existing == step {
				return
			}
		}
		steps = append(steps, step)
	}
	resource := strings.ToLower(req.Spec.Workload.Kind)
	if resource == "" {
		resource = "deployment"
	}
	baseDescribe := fmt.Sprintf("inspect %s status with kubectl describe %s/%s -n %s", resource, resource, req.Spec.Workload.Name, req.Spec.Workload.Namespace)
	switch focus {
	case FocusStartupFailure:
		appendIfMissing("inspect the newest pod logs for startup failures")
		appendIfMissing(baseDescribe)
		appendIfMissing("review recent image or configuration changes")
	case FocusConfigOrDependency:
		appendIfMissing("inspect recent configuration and dependency changes")
		appendIfMissing(baseDescribe)
		appendIfMissing("verify referenced ConfigMaps, Secrets, and upstream dependencies")
	case FocusSafetyBlocked:
		appendIfMissing("review the blocking safety reason before retrying any action")
		appendIfMissing(baseDescribe)
		appendIfMissing("do not continue automatic retries until the block condition is cleared")
	case FocusManualFollowUp:
		appendIfMissing(baseDescribe)
		appendIfMissing("inspect the newest pod logs and recent runtime events")
		appendIfMissing("decide whether to retry manually after reviewing the incident context")
	case FocusTransientRecovered:
		appendIfMissing("continue passive observation for repeated incidents")
		appendIfMissing("review Grafana trends if the same alert appears again")
	case FocusInsufficientEvidence:
		appendIfMissing(baseDescribe)
		appendIfMissing("inspect recent runtime events and gather the newest pod logs")
	}
	if len(steps) > 3 {
		steps = steps[:3]
	}
	return steps
}

func buildHandoff(req *ksv1alpha1.HealingRequest, focus Focus) string {
	timestamp := "unknown-time"
	if !req.CreationTimestamp.IsZero() {
		timestamp = req.CreationTimestamp.UTC().Format(time.RFC3339)
	}
	trigger := strings.TrimSpace(req.Annotations["kube-sentinel.io/alert-category"])
	if trigger == "" {
		trigger = strings.TrimSpace(req.Status.LastEventReason)
	}
	parts := []string{fmt.Sprintf("%s incident on %s/%s (%s)", timestamp, req.Spec.Workload.Namespace, req.Spec.Workload.Name, req.Spec.Workload.Kind)}
	if trigger != "" {
		parts = append(parts, fmt.Sprintf("trigger: %s", trigger))
	}
	parts = append(parts, fmt.Sprintf("focus: %s", focus))
	if req.Status.LastAction != "" {
		parts = append(parts, fmt.Sprintf("runtime action: %s", req.Status.LastAction))
	}
	if req.Status.NextRecommendation != "" {
		parts = append(parts, fmt.Sprintf("next step: %s", req.Status.NextRecommendation))
	}
	if req.Status.CorrelationKey != "" {
		parts = append(parts, fmt.Sprintf("correlation: %s", req.Status.CorrelationKey))
	}
	return strings.Join(parts, "; ")
}

func buildSummary(req *ksv1alpha1.HealingRequest, trigger string, focus Focus) string {
	parts := []string{fmt.Sprintf("workload=%s/%s", req.Spec.Workload.Namespace, req.Spec.Workload.Name)}
	if req.Spec.Workload.Kind != "" {
		parts = append(parts, fmt.Sprintf("kind=%s", req.Spec.Workload.Kind))
	}
	if trigger != "" {
		parts = append(parts, fmt.Sprintf("trigger=%s", trigger))
	}
	if req.Status.Phase != "" {
		parts = append(parts, fmt.Sprintf("phase=%s", req.Status.Phase))
	}
	if req.Status.LastAction != "" {
		parts = append(parts, fmt.Sprintf("action=%s", req.Status.LastAction))
	}
	parts = append(parts, fmt.Sprintf("focus=%s", focus))
	if req.Status.CorrelationKey != "" {
		parts = append(parts, fmt.Sprintf("correlation=%s", req.Status.CorrelationKey))
	}
	return strings.Join(parts, "; ")
}

func buildTelegramNotification(req *ksv1alpha1.HealingRequest, trigger string, report Report) Notification {
	kind := notificationKindFor(req)
	workload := fmt.Sprintf("%s/%s", req.Spec.Workload.Namespace, req.Spec.Workload.Name)
	var short string
	var long string
	statusLine := fmt.Sprintf("status: %s", req.Status.Phase)
	entryPoints := []string{
		fmt.Sprintf("object: HealingRequest %s", req.Name),
		fmt.Sprintf("trend: Grafana namespace=%s workload=%s", req.Spec.Workload.Namespace, req.Spec.Workload.Name),
		fmt.Sprintf("precise: kubectl describe %s/%s -n %s", strings.ToLower(req.Spec.Workload.Kind), req.Spec.Workload.Name, req.Spec.Workload.Namespace),
	}
	switch kind {
	case NotificationAutoTried:
		short = fmt.Sprintf("[INFO] %s auto-tried; %s", workload, trigger)
		long = fmt.Sprintf("[INFO] Sentinel auto-tried recovery\n\n%s\n%s\ncurrent focus: %s\nnext: %s\ncorrelation: %s\nwhere next:\n- %s\n- %s\n- %s", strings.Join(report.WhatHappened, "\n"), strings.Join(report.WhatRuntimeDid, "\n"), report.CurrentFocus, strings.Join(report.NextSteps, "; "), req.Status.CorrelationKey, entryPoints[0], entryPoints[1], entryPoints[2])
	case NotificationRecovered:
		short = fmt.Sprintf("[OK] %s recovered; no immediate action", workload)
		long = fmt.Sprintf("[OK] Sentinel observation complete\n\n%s\n%s\ncurrent focus: %s\nnext: %s\ncorrelation: %s\nwhere next:\n- %s\n- %s", strings.Join(report.WhatHappened, "\n"), statusLine, report.CurrentFocus, strings.Join(report.NextSteps, "; "), req.Status.CorrelationKey, entryPoints[0], entryPoints[1])
	default:
		short = fmt.Sprintf("[WARN] %s blocked; %s", workload, strings.TrimSpace(req.Status.BlockReasonCode))
		long = fmt.Sprintf("[WARN] Sentinel blocked automatic handling\n\n%s\n%s\ncurrent focus: %s\nwhy: %s\nnext: %s\ncorrelation: %s\nwhere next:\n- %s\n- %s\n- %s", strings.Join(report.WhatHappened, "\n"), strings.Join(report.WhatRuntimeDid, "\n"), report.CurrentFocus, report.FocusDetail, strings.Join(report.NextSteps, "; "), req.Status.CorrelationKey, entryPoints[0], entryPoints[1], entryPoints[2])
	}
	return Notification{Kind: kind, ShortMessage: short, LongMessage: long}
}

func notificationKindFor(req *ksv1alpha1.HealingRequest) NotificationKind {
	if req == nil {
		return NotificationBlocked
	}
	if req.Status.Phase == ksv1alpha1.PhaseSuppressed || strings.EqualFold(strings.TrimSpace(req.Annotations["kube-sentinel.io/alert-status"]), "resolved") {
		return NotificationRecovered
	}
	if req.Status.Phase == ksv1alpha1.PhaseCompleted && strings.Contains(req.Status.LastAction, "deployment-l1") {
		return NotificationAutoTried
	}
	return NotificationBlocked
}
