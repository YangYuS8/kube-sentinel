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
}

func (o *Orchestrator) Process(ctx context.Context, req *ksv1alpha1.HealingRequest) error {
	req.ApplyDefaults()
	if err := req.Validate(); err != nil {
		req.Status.LastError = err.Error()
		req.Status.Phase = ksv1alpha1.PhaseBlocked
		return err
	}
	if req.Status.ObservedGeneration == req.Generation && req.Status.Phase == ksv1alpha1.PhaseCompleted {
		return nil
	}
	if !o.Adapter.Supports(req.Spec.Workload.Kind) {
		req.Status.LastError = "unsupported kind"
		req.Status.Phase = ksv1alpha1.PhaseBlocked
		return fmt.Errorf("unsupported kind")
	}
	if o.Metrics != nil {
		o.Metrics.IncTriggers()
	}
	if o.Breaker != nil {
		allow, reason := o.Breaker.Allow(req.Spec.Workload.Namespace+"/"+req.Spec.Workload.Name, time.Now())
		if !allow {
			req.Status.Phase = ksv1alpha1.PhaseBlocked
			req.Status.LastError = reason
			req.Status.CircuitBreaker.ObjectOpen = true
			if o.Metrics != nil {
				o.Metrics.IncCircuitBreaks()
			}
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
		return fmt.Errorf(decision.Reason)
	}

	snap, err := o.Snapshotter.Create(req.Spec.Workload.Namespace, req.Spec.Workload.Name)
	if err != nil {
		req.Status.Phase = ksv1alpha1.PhaseBlocked
		req.Status.LastError = err.Error()
		return err
	}

	req.Status.Phase = ksv1alpha1.PhaseL1
	revisions, err := o.Adapter.ListRevisions(ctx, req.Spec.Workload.Namespace, req.Spec.Workload.Name)
	if err != nil {
		req.Status.LastError = err.Error()
		if o.Breaker != nil {
			o.Breaker.RecordFailure(req.Spec.Workload.Namespace+"/"+req.Spec.Workload.Name, time.Now())
		}
		return err
	}
	latest, err := SelectLatestHealthyRevision(revisions)
	if err != nil {
		req.Status.Phase = ksv1alpha1.PhaseL3
		req.Status.LastError = "no healthy revision"
		req.Status.LastAction = "manual-intervention"
		return nil
	}
	if err := o.Adapter.RollbackToRevision(ctx, req.Spec.Workload.Namespace, req.Spec.Workload.Name, latest.Revision); err != nil {
		_ = o.Snapshotter.Restore(snap)
		req.Status.Phase = ksv1alpha1.PhaseBlocked
		req.Status.LastError = err.Error()
		return err
	}

	req.Status.Phase = ksv1alpha1.PhaseCompleted
	req.Status.LastAction = "rollback-to-healthy"
	req.Status.LastHealthyRevision = latest.Revision
	req.Status.ObservedGeneration = req.Generation
	if o.Metrics != nil {
		o.Metrics.IncRollbacks()
		o.Metrics.IncSuccess()
	}
	if o.AuditSink != nil {
		o.AuditSink.Write(observability.AuditEvent{
			ID:          req.Name,
			Trigger:     "alertmanager",
			Target:      req.Spec.Workload.Namespace + "/" + req.Spec.Workload.Name,
			BeforeState: "unhealthy",
			AfterState:  "rolled-back",
			Result:      "success",
			CreatedAt:   time.Now(),
		})
	}
	return nil
}
