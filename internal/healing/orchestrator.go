package healing

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	ksv1alpha1 "github.com/yangyus8/kube-sentinel/api/v1alpha1"
	"github.com/yangyus8/kube-sentinel/internal/observability"
	"github.com/yangyus8/kube-sentinel/internal/safety"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Orchestrator struct {
	Adapter              WorkloadAdapter
	Snapshotter          Snapshotter
	Breaker              *safety.CircuitBreaker
	Metrics              *observability.Metrics
	AuditSink            observability.AuditSink
	EventSink            observability.EventSink
	RuntimeInputProvider RuntimeInputProvider
	K8sEventRecorder     record.EventRecorder
	Now                  func() time.Time

	mu              sync.Mutex
	breakersByScope map[string]*safety.CircuitBreaker
	actionHistory   map[string][]time.Time
}

func (o *Orchestrator) Process(ctx context.Context, req *ksv1alpha1.HealingRequest) error {
	if o.Now == nil {
		o.Now = time.Now
	}
	startedAt := o.Now()
	defer func() {
		if o.Metrics != nil {
			o.Metrics.ObserveStrategyDuration("process", o.Now().Sub(startedAt))
		}
	}()
	req.ApplyDefaults()
	req.Status.CorrelationKey = req.Annotations["kube-sentinel.io/correlation-key"]
	logger := log.FromContext(ctx).WithValues(
		"workloadNamespace", req.Spec.Workload.Namespace,
		"workloadName", req.Spec.Workload.Name,
		"workloadKind", req.Spec.Workload.Kind,
		"correlationKey", req.Status.CorrelationKey,
	)
	if err := req.Validate(); err != nil {
		logger.Error(err, "healing request validation failed")
		if o.Metrics != nil {
			o.Metrics.IncFailures()
		}
		req.Status.LastError = err.Error()
		req.Status.Phase = ksv1alpha1.PhaseBlocked
		req.Status.LastEventReason = "validation-failed"
		return err
	}
	if req.Status.ObservedGeneration == req.Generation && req.Status.Phase == ksv1alpha1.PhaseCompleted {
		return nil
	}
	if !o.Adapter.Supports(req.Spec.Workload.Kind) {
		logger.Info("unsupported workload kind blocked", "kind", req.Spec.Workload.Kind)
		if o.Metrics != nil {
			o.Metrics.IncFailures()
		}
		req.Status.LastError = "unsupported kind"
		req.Status.Phase = ksv1alpha1.PhaseBlocked
		req.Status.LastEventReason = "unsupported-workload"
		return fmt.Errorf("unsupported kind")
	}
	if o.Metrics != nil {
		o.Metrics.IncTriggers()
	}
	req.Status.LastEventReason = "ingested"
	breaker := o.breakerFor(req)
	if breaker != nil {
		allow, reason := breaker.Allow(req.Spec.Workload.Namespace+"/"+req.Spec.Workload.Name, o.Now())
		if !allow {
			logger.Info("circuit breaker blocked request", "reason", reason)
			if o.Metrics != nil {
				o.Metrics.IncFailures()
			}
			req.Status.Phase = ksv1alpha1.PhaseBlocked
			req.Status.LastError = reason
			req.Status.LastGateDecision = reason
			state := breaker.Status(req.Spec.Workload.Namespace + "/" + req.Spec.Workload.Name)
			req.Status.CircuitBreaker.ObjectOpen = state.OpenReason == "object breaker open"
			req.Status.CircuitBreaker.DomainOpen = state.OpenReason == "domain breaker open"
			req.Status.CircuitBreaker.CurrentObjectFailures = state.CurrentObjectFailures
			req.Status.CircuitBreaker.CurrentDomainFailures = state.CurrentDomainFailures
			req.Status.CircuitBreaker.RecoveryAt = state.RecoveryAt
			req.Status.CircuitBreaker.OpenReason = fmt.Sprintf("%s (objectThreshold=%d, domainThreshold=%d)", state.OpenReason, req.Spec.CircuitBreaker.ObjectFailureThreshold, req.Spec.CircuitBreaker.DomainFailureThreshold)
			if o.Metrics != nil {
				o.Metrics.IncCircuitBreaks()
			}
			o.emitRuntimeEvent(req, "Warning", "CircuitBreakerOpen", reason)
			o.writeAudit(req, "blocked", req.Status.CircuitBreaker.OpenReason)
			return errors.New(reason)
		}
	}
	runtimeInputProvider := o.RuntimeInputProvider
	if runtimeInputProvider == nil {
		runtimeInputProvider = adapterRuntimeInputProvider{adapter: o.Adapter}
	}
	runtimeInput, err := runtimeInputProvider.Build(ctx, req)
	if err != nil {
		logger.Error(err, "runtime gate input unavailable")
		if o.Metrics != nil {
			o.Metrics.IncFailures()
		}
		req.Status.Phase = ksv1alpha1.PhaseBlocked
		req.Status.LastError = fmt.Sprintf("runtime input unavailable: %v", err)
		req.Status.LastGateDecision = "runtime input unavailable"
		o.emitRuntimeEvent(req, "Warning", "GateInputUnavailable", err.Error())
		o.writeAudit(req, "blocked", req.Status.LastError)
		return err
	}
	runtimeInput.ActionsInWindow = o.actionsInWindow(req.Spec.Workload.Namespace+"/"+req.Spec.Workload.Name, o.Now(), req.Spec.RateLimit.WindowMinutes)
	decision := safety.Evaluate(safety.GateInput{
		Now:                o.Now(),
		MaintenanceWindows: req.Spec.MaintenanceWindows,
		ActionsInWindow:    runtimeInput.ActionsInWindow,
		MaxActions:         req.Spec.RateLimit.MaxActions,
		AffectedPods:       runtimeInput.AffectedPods,
		ClusterPods:        runtimeInput.ClusterPods,
		MaxPodPercentage:   req.Spec.BlastRadius.MaxPodPercentage,
	})
	if !decision.Allow {
		logger.Info("gate blocked request", "reason", decision.Reason, "actionsInWindow", runtimeInput.ActionsInWindow, "maxActions", req.Spec.RateLimit.MaxActions)
		if o.Metrics != nil {
			o.Metrics.IncFailures()
			if decision.Reason == "maintenance window" {
				o.Metrics.IncMaintenanceWindowConflicts()
			}
		}
		req.Status.Phase = ksv1alpha1.PhaseBlocked
		req.Status.LastError = decision.Reason
		req.Status.LastGateDecision = fmt.Sprintf("%s (actions=%d,max=%d,affectedPods=%d,clusterPods=%d,maxPodPct=%d)", decision.Reason, runtimeInput.ActionsInWindow, req.Spec.RateLimit.MaxActions, runtimeInput.AffectedPods, runtimeInput.ClusterPods, req.Spec.BlastRadius.MaxPodPercentage)
		o.emitRuntimeEvent(req, "Warning", "GateBlocked", req.Status.LastGateDecision)
		o.writeAudit(req, "blocked", req.Status.LastGateDecision)
		return errors.New(decision.Reason)
	}
	o.recordActionAttempt(req.Spec.Workload.Namespace+"/"+req.Spec.Workload.Name, o.Now(), req.Spec.RateLimit.WindowMinutes)
	req.Status.LastGateDecision = "allowed"

	snap, err := o.Snapshotter.Create(req.Spec.Workload.Namespace, req.Spec.Workload.Name)
	if err != nil {
		logger.Error(err, "snapshot creation failed")
		if o.Metrics != nil {
			o.Metrics.IncFailures()
		}
		req.Status.Phase = ksv1alpha1.PhaseBlocked
		req.Status.LastError = err.Error()
		req.Status.LastEventReason = "snapshot-failed"
		return err
	}

	req.Status.Phase = ksv1alpha1.PhaseL1
	revisions, err := o.Adapter.ListRevisions(ctx, req.Spec.Workload.Namespace, req.Spec.Workload.Name)
	if err != nil {
		logger.Error(err, "list revisions failed")
		if o.Metrics != nil {
			o.Metrics.IncFailures()
		}
		req.Status.LastError = err.Error()
		if breaker != nil {
			breaker.RecordFailure(req.Spec.Workload.Namespace+"/"+req.Spec.Workload.Name, o.Now())
			state := breaker.Status(req.Spec.Workload.Namespace + "/" + req.Spec.Workload.Name)
			req.Status.CircuitBreaker.ObjectOpen = state.OpenReason == "object breaker open"
			req.Status.CircuitBreaker.DomainOpen = state.OpenReason == "domain breaker open"
			req.Status.CircuitBreaker.CurrentObjectFailures = state.CurrentObjectFailures
			req.Status.CircuitBreaker.CurrentDomainFailures = state.CurrentDomainFailures
			req.Status.CircuitBreaker.RecoveryAt = state.RecoveryAt
			req.Status.CircuitBreaker.OpenReason = fmt.Sprintf("%s (objectThreshold=%d, domainThreshold=%d)", state.OpenReason, req.Spec.CircuitBreaker.ObjectFailureThreshold, req.Spec.CircuitBreaker.DomainFailureThreshold)
		}
		req.Status.LastEventReason = "revision-list-failed"
		o.emitRuntimeEvent(req, "Warning", "RevisionListFailed", err.Error())
		o.writeAudit(req, "failed", err.Error())
		return err
	}
	latest, err := SelectLatestHealthyRevision(revisions)
	if err != nil {
		logger.Info("no healthy revision, fallback to L3", "reason", err.Error())
		if o.Metrics != nil {
			o.Metrics.IncFailures()
		}
		req.Status.Phase = ksv1alpha1.PhaseL3
		req.Status.LastError = "no healthy revision available; inspect deployment revision evidence and alert history"
		req.Status.LastEvidenceStatus = "insufficient-evidence"
		req.Status.LastAction = "manual-intervention"
		req.Status.LastEventReason = "no-healthy-revision"
		o.emitRuntimeEvent(req, "Warning", "L3Fallback", req.Status.LastError)
		o.writeAudit(req, "fallback", req.Status.LastError)
		return nil
	}
	req.Status.LastEvidenceStatus = "healthy-revision-selected"
	if err := o.Adapter.RollbackToRevision(ctx, req.Spec.Workload.Namespace, req.Spec.Workload.Name, latest.Revision); err != nil {
		logger.Error(err, "rollback to healthy revision failed", "revision", latest.Revision)
		if o.Metrics != nil {
			o.Metrics.IncFailures()
		}
		_ = o.Snapshotter.Restore(snap)
		req.Status.Phase = ksv1alpha1.PhaseBlocked
		req.Status.LastError = err.Error()
		req.Status.LastEventReason = "rollback-failed"
		o.emitRuntimeEvent(req, "Warning", "RollbackFailed", err.Error())
		o.writeAudit(req, "failed", fmt.Sprintf("rollback failed and restored snapshot: %v", err))
		return err
	}

	req.Status.Phase = ksv1alpha1.PhaseCompleted
	logger.Info("healing process completed", "healthyRevision", latest.Revision)
	req.Status.LastAction = "rollback-to-healthy"
	req.Status.LastHealthyRevision = latest.Revision
	req.Status.LastEventReason = "rollback-succeeded"
	req.Status.ObservedGeneration = req.Generation
	if o.Metrics != nil {
		o.Metrics.IncRollbacks()
		o.Metrics.IncSuccess()
	}
	o.writeAudit(req, "success", "rolled-back")
	o.emitRuntimeEvent(req, "Normal", "ClosedLoopCompleted", "runtime closed-loop completed")
	return nil
}

func (o *Orchestrator) breakerFor(req *ksv1alpha1.HealingRequest) *safety.CircuitBreaker {
	if o.Breaker != nil {
		return o.Breaker
	}
	scopeKey := "global"
	if req.Spec.CircuitBreaker.Scope == ksv1alpha1.BreakerScopeNamespace {
		scopeKey = "ns:" + req.Spec.Workload.Namespace
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.breakersByScope == nil {
		o.breakersByScope = map[string]*safety.CircuitBreaker{}
	}
	if existing := o.breakersByScope[scopeKey]; existing != nil {
		return existing
	}
	created := safety.NewCircuitBreaker(
		req.Spec.CircuitBreaker.ObjectFailureThreshold,
		req.Spec.CircuitBreaker.DomainFailureThreshold,
		req.Spec.CircuitBreaker.CooldownMinutes,
	)
	o.breakersByScope[scopeKey] = created
	return created
}

func (o *Orchestrator) recordActionAttempt(objectKey string, now time.Time, windowMinutes int) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.actionHistory == nil {
		o.actionHistory = map[string][]time.Time{}
	}
	window := time.Duration(windowMinutes) * time.Minute
	existing := o.actionHistory[objectKey]
	kept := make([]time.Time, 0, len(existing)+1)
	for _, ts := range existing {
		if now.Sub(ts) <= window {
			kept = append(kept, ts)
		}
	}
	kept = append(kept, now)
	o.actionHistory[objectKey] = kept
}

func (o *Orchestrator) actionsInWindow(objectKey string, now time.Time, windowMinutes int) int {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.actionHistory == nil {
		o.actionHistory = map[string][]time.Time{}
	}
	window := time.Duration(windowMinutes) * time.Minute
	existing := o.actionHistory[objectKey]
	kept := make([]time.Time, 0, len(existing))
	for _, ts := range existing {
		if now.Sub(ts) <= window {
			kept = append(kept, ts)
		}
	}
	o.actionHistory[objectKey] = kept
	return len(kept)
}

func (o *Orchestrator) writeAudit(req *ksv1alpha1.HealingRequest, result, afterState string) {
	if o.AuditSink == nil {
		return
	}
	o.AuditSink.Write(observability.AuditEvent{
		ID:          req.Status.CorrelationKey,
		Trigger:     "alertmanager",
		Target:      req.Spec.Workload.Namespace + "/" + req.Spec.Workload.Name,
		BeforeState: req.Status.LastEventReason,
		AfterState:  afterState,
		Result:      result,
		CreatedAt:   o.Now(),
	})
}

func (o *Orchestrator) emitRuntimeEvent(req *ksv1alpha1.HealingRequest, eventType, reason, message string) {
	if o.EventSink == nil {
		return
	}
	o.EventSink.Record(observability.RuntimeEvent{
		CorrelationKey: req.Status.CorrelationKey,
		Namespace:      req.Spec.Workload.Namespace,
		Name:           req.Spec.Workload.Name,
		ResourceKind:   req.Spec.Workload.Kind,
		Reason:         reason,
		Message:        message,
		Type:           eventType,
		CreatedAt:      o.Now(),
	})
	if o.K8sEventRecorder != nil {
		o.K8sEventRecorder.Eventf(req, eventType, reason, "%s", message)
	}
}
