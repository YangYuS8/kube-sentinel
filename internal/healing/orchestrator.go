package healing

import (
	"context"
	"fmt"
	"time"

	ksv1alpha1 "github.com/yangyus8/kube-sentinel/api/v1alpha1"
	"github.com/yangyus8/kube-sentinel/internal/observability"
	"github.com/yangyus8/kube-sentinel/internal/safety"
)

type Orchestrator struct {
	Adapter    WorkloadAdapter
	Snapshotter Snapshotter
	Breaker    *safety.CircuitBreaker
	Metrics    *observability.Metrics
	AuditSink  observability.AuditSink
	EventSink  observability.EventSink
	Now        func() time.Time
}

func (o *Orchestrator) Process(ctx context.Context, req *ksv1alpha1.HealingRequest) error {
	if o.Now == nil {
		o.Now = time.Now
	}
	req.ApplyDefaults()
	req.Status.CorrelationKey = req.Annotations["kube-sentinel.io/correlation-key"]
	if err := req.Validate(); err != nil {
		req.Status.LastError = err.Error()
		req.Status.Phase = ksv1alpha1.PhaseBlocked
		req.Status.LastEventReason = "validation-failed"
		return err
	}
	if req.Status.ObservedGeneration == req.Generation && req.Status.Phase == ksv1alpha1.PhaseCompleted {
		return nil
	}
	if !o.Adapter.Supports(req.Spec.Workload.Kind) {
		req.Status.LastError = "unsupported kind"
		req.Status.Phase = ksv1alpha1.PhaseBlocked
		req.Status.LastEventReason = "unsupported-workload"
		return fmt.Errorf("unsupported kind")
	}
	if o.Metrics != nil {
		o.Metrics.IncTriggers()
	}
	req.Status.LastEventReason = "ingested"
	if o.Breaker != nil {
		allow, reason := o.Breaker.Allow(req.Spec.Workload.Namespace+"/"+req.Spec.Workload.Name, o.Now())
		if !allow {
			req.Status.Phase = ksv1alpha1.PhaseBlocked
			req.Status.LastError = reason
			req.Status.LastGateDecision = reason
			req.Status.CircuitBreaker.ObjectOpen = true
			state := o.Breaker.Status(req.Spec.Workload.Namespace + "/" + req.Spec.Workload.Name)
			req.Status.CircuitBreaker.CurrentObjectFailures = state.CurrentObjectFailures
			req.Status.CircuitBreaker.CurrentDomainFailures = state.CurrentDomainFailures
			req.Status.CircuitBreaker.RecoveryAt = state.RecoveryAt
			req.Status.CircuitBreaker.OpenReason = state.OpenReason
			if o.Metrics != nil {
				o.Metrics.IncCircuitBreaks()
			}
			o.emitRuntimeEvent(req, "Warning", "CircuitBreakerOpen", reason)
			return fmt.Errorf(reason)
		}
	}
	decision := safety.Evaluate(safety.GateInput{
		Now:                time.Now(),
		MaintenanceWindows: req.Spec.MaintenanceWindows,
		ActionsInWindow:    0,
		MaxActions:         req.Spec.RateLimit.MaxActions,
		AffectedPods:       1,
		ClusterPods:        20,
	})
	if !decision.Allow {
		req.Status.Phase = ksv1alpha1.PhaseBlocked
		req.Status.LastError = decision.Reason
		req.Status.LastGateDecision = decision.Reason
		o.emitRuntimeEvent(req, "Warning", "GateBlocked", decision.Reason)
		return fmt.Errorf(decision.Reason)
	}
	req.Status.LastGateDecision = "allowed"

	snap, err := o.Snapshotter.Create(req.Spec.Workload.Namespace, req.Spec.Workload.Name)
	if err != nil {
		req.Status.Phase = ksv1alpha1.PhaseBlocked
		req.Status.LastError = err.Error()
		req.Status.LastEventReason = "snapshot-failed"
		return err
	}

	req.Status.Phase = ksv1alpha1.PhaseL1
	revisions, err := o.Adapter.ListRevisions(ctx, req.Spec.Workload.Namespace, req.Spec.Workload.Name)
	if err != nil {
		req.Status.LastError = err.Error()
		if o.Breaker != nil {
			o.Breaker.RecordFailure(req.Spec.Workload.Namespace+"/"+req.Spec.Workload.Name, o.Now())
			state := o.Breaker.Status(req.Spec.Workload.Namespace + "/" + req.Spec.Workload.Name)
			req.Status.CircuitBreaker.CurrentObjectFailures = state.CurrentObjectFailures
			req.Status.CircuitBreaker.CurrentDomainFailures = state.CurrentDomainFailures
			req.Status.CircuitBreaker.RecoveryAt = state.RecoveryAt
			req.Status.CircuitBreaker.OpenReason = state.OpenReason
		}
		req.Status.LastEventReason = "revision-list-failed"
		o.emitRuntimeEvent(req, "Warning", "RevisionListFailed", err.Error())
		return err
	}
	latest, err := SelectLatestHealthyRevision(revisions)
	if err != nil {
		req.Status.Phase = ksv1alpha1.PhaseL3
		req.Status.LastError = err.Error()
		req.Status.LastEvidenceStatus = "insufficient-evidence"
		req.Status.LastAction = "manual-intervention"
		req.Status.LastEventReason = "no-healthy-revision"
		o.emitRuntimeEvent(req, "Warning", "L3Fallback", err.Error())
		return nil
	}
	req.Status.LastEvidenceStatus = "healthy-revision-selected"
	if err := o.Adapter.RollbackToRevision(ctx, req.Spec.Workload.Namespace, req.Spec.Workload.Name, latest.Revision); err != nil {
		_ = o.Snapshotter.Restore(snap)
		req.Status.Phase = ksv1alpha1.PhaseBlocked
		req.Status.LastError = err.Error()
		req.Status.LastEventReason = "rollback-failed"
		o.emitRuntimeEvent(req, "Warning", "RollbackFailed", err.Error())
		return err
	}

	req.Status.Phase = ksv1alpha1.PhaseCompleted
	req.Status.LastAction = "rollback-to-healthy"
	req.Status.LastHealthyRevision = latest.Revision
	req.Status.LastEventReason = "rollback-succeeded"
	req.Status.ObservedGeneration = req.Generation
	if o.Metrics != nil {
		o.Metrics.IncRollbacks()
		o.Metrics.IncSuccess()
	}
	if o.AuditSink != nil {
		o.AuditSink.Write(observability.AuditEvent{
			ID:          req.Status.CorrelationKey,
			Trigger:     "alertmanager",
			Target:      req.Spec.Workload.Namespace + "/" + req.Spec.Workload.Name,
			BeforeState: "unhealthy",
			AfterState:  "rolled-back",
			Result:      "success",
			CreatedAt:   o.Now(),
		})
	}
	o.emitRuntimeEvent(req, "Normal", "ClosedLoopCompleted", "runtime closed-loop completed")
	return nil
}

func (o *Orchestrator) emitRuntimeEvent(req *ksv1alpha1.HealingRequest, eventType, reason, message string) {
	if o.EventSink == nil {
		return
	}
	o.EventSink.Record(observability.RuntimeEvent{
		CorrelationKey: req.Status.CorrelationKey,
		Reason:         reason,
		Message:        message,
		Type:           eventType,
		CreatedAt:      o.Now(),
	})
}
