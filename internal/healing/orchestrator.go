package healing

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	req.Status.WorkloadCapability = workloadCapabilityForRequest(req)
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
		req.Status.BlockReasonCode = "unsupported_workload"
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
			req.Status.BlockReasonCode = "circuit_breaker_open"
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
		req.Status.BlockReasonCode = "runtime_input_unavailable"
		req.Status.LastGateDecision = "runtime input unavailable"
		o.emitRuntimeEvent(req, "Warning", "GateInputUnavailable", err.Error())
		o.writeAudit(req, "blocked", req.Status.LastError)
		return err
	}
	runtimeInput.ActionsInWindow = o.actionsInWindow(req.Spec.Workload.Namespace+"/"+req.Spec.Workload.Name, o.Now(), req.Spec.RateLimit.WindowMinutes)
	if hasAlertMetadata(req) && isResolvedAlert(req) {
		req.Status.Phase = ksv1alpha1.PhaseSuppressed
		req.Status.SuppressedAt = o.Now().Format(time.RFC3339)
		req.Status.LastAction = "suppressed"
		req.Status.LastEventReason = "suppressed-during-soak"
		req.Status.LastEvidenceStatus = "suppressed"
		o.emitRuntimeEvent(req, "Normal", "Suppressed", "alert recovered during observation window")
		o.writeAudit(req, "suppressed", "alert recovered during observation window")
		if o.Metrics != nil {
			o.Metrics.IncSuppressed()
		}
		return nil
	}
	if hasAlertMetadata(req) {
		soakDuration, minSamples := soakProfileFor(req)
		if pending, done := o.advanceSoakWindow(req, soakDuration, minSamples); !done {
			if pending {
				o.emitRuntimeEvent(req, "Normal", "PendingVerify", req.Status.LastGateDecision)
				o.writeAudit(req, "pending-verify", req.Status.LastGateDecision)
			}
			return nil
		}
	}
	totalWorkloads := maxInt(runtimeInput.TotalWorkloads, 1)
	unhealthyWorkloads := runtimeInput.UnhealthyWorkloads
	namespaceBlockRate := unhealthyWorkloads * 100 / totalWorkloads
	req.Status.NamespaceBlockRate = namespaceBlockRate
	blockedByNamespaceBudget := false
	if totalWorkloads < req.Spec.NamespaceBudget.MinTotalWorkloads {
		blockedByNamespaceBudget = unhealthyWorkloads >= req.Spec.NamespaceBudget.FallbackUnhealthyCount
	} else {
		blockedByNamespaceBudget = namespaceBlockRate >= req.Spec.NamespaceBudget.BlockingThresholdPercent
	}
	if blockedByNamespaceBudget {
		if req.Spec.EmergencyTry.Enabled && isCriticalWorkload(req) && req.Status.EmergencyAttempts < req.Spec.EmergencyTry.MaxAttempts {
			req.Status.EmergencyAttempts++
			req.Status.ShadowAction = "namespace budget blocked, emergency bypass granted"
			o.emitRuntimeEvent(req, "Warning", "EmergencyBypass", req.Status.ShadowAction)
		} else {
			if o.Metrics != nil {
				o.Metrics.IncFailures()
				o.Metrics.IncReadOnlyBlocks("namespace_budget", req.Spec.Workload.Kind)
			}
			req.Status.Phase = ksv1alpha1.PhaseBlocked
			req.Status.LastError = "namespace budget blocked"
			req.Status.BlockReasonCode = "namespace_budget_blocked"
			req.Status.ShadowAction = "would execute rollback-to-healthy but blocked by namespace budget"
			req.Status.LastGateDecision = fmt.Sprintf("namespace budget exceeded (rate=%d%%, unhealthy=%d, total=%d)", namespaceBlockRate, unhealthyWorkloads, totalWorkloads)
			o.emitRuntimeEvent(req, "Warning", "NamespaceBudgetBlocked", req.Status.LastGateDecision)
			o.writeAudit(req, "blocked", req.Status.LastGateDecision)
			return errors.New(req.Status.LastError)
		}
	}
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
		req.Status.BlockReasonCode = "gate_blocked"
		req.Status.ShadowAction = "would execute rollback-to-healthy but blocked by gate"
		req.Status.LastGateDecision = fmt.Sprintf("%s (actions=%d,max=%d,affectedPods=%d,clusterPods=%d,maxPodPct=%d)", decision.Reason, runtimeInput.ActionsInWindow, req.Spec.RateLimit.MaxActions, runtimeInput.AffectedPods, runtimeInput.ClusterPods, req.Spec.BlastRadius.MaxPodPercentage)
		o.emitRuntimeEvent(req, "Warning", "GateBlocked", req.Status.LastGateDecision)
		o.writeAudit(req, "blocked", req.Status.LastGateDecision)
		if o.Metrics != nil {
			o.Metrics.IncReadOnlyBlocks("gate", req.Spec.Workload.Kind)
		}
		return errors.New(decision.Reason)
	}
	req.Status.LastGateDecision = "allowed"

	if req.Spec.Workload.Kind == "StatefulSet" {
		if frozen, freezeReason := isStatefulSetFrozen(req, o.Now()); frozen {
			if o.Metrics != nil {
				o.Metrics.IncFailures()
				o.Metrics.IncReadOnlyBlocks("statefulset_frozen", req.Spec.Workload.Kind)
				o.Metrics.IncStatefulSetControlledAction(req.Spec.Workload.Kind, "restart", "blocked", "frozen")
			}
			req.Status.Phase = ksv1alpha1.PhaseBlocked
			req.Status.LastError = freezeReason
			req.Status.BlockReasonCode = "statefulset_frozen"
			req.Status.LastAction = "manual-intervention"
			req.Status.LastEventReason = "statefulset-frozen"
			req.Status.StatefulSetFreezeState = "frozen"
			req.Status.StatefulSetFailureReason = freezeReason
			req.Status.ShadowAction = "would execute controlled statefulset action but blocked by freeze window"
			o.emitRuntimeEvent(req, "Warning", "StatefulSetFrozen", freezeReason)
			o.writeAudit(req, "blocked", freezeReason)
			return errors.New(req.Status.LastError)
		}
		authorized, authReason := authorizeStatefulSet(req, runtimeInput)
		req.Status.StatefulSetAuthorization = authReason
		if !authorized {
			blockReasonCode := "statefulset_authorization_failed"
			lastEventReason := "statefulset-authorization-failed"
			runtimeEventReason := "StatefulSetAuthorizationFailed"
			if authReason == "statefulset policy is read-only" {
				blockReasonCode = "statefulset_readonly"
				lastEventReason = "statefulset-readonly"
				runtimeEventReason = "StatefulSetReadOnlyBlocked"
			}
			if o.Metrics != nil {
				o.Metrics.IncFailures()
				o.Metrics.IncReadOnlyBlocks("statefulset_authorization", req.Spec.Workload.Kind)
				o.Metrics.IncStatefulSetControlledAction(req.Spec.Workload.Kind, "restart", "blocked", "none")
			}
			req.Status.Phase = ksv1alpha1.PhaseBlocked
			req.Status.LastError = "statefulset controlled action is not authorized; manual intervention required"
			req.Status.BlockReasonCode = blockReasonCode
			req.Status.LastAction = "manual-intervention"
			req.Status.LastEventReason = lastEventReason
			req.Status.ShadowAction = "would execute controlled statefulset action but authorization gate failed"
			o.emitRuntimeEvent(req, "Warning", runtimeEventReason, authReason)
			o.writeAudit(req, "blocked", authReason)
			return errors.New(req.Status.LastError)
		}
		if runtimeInput.ActionsInWindow > 0 {
			if o.Metrics != nil {
				o.Metrics.IncFailures()
				o.Metrics.IncReadOnlyBlocks("statefulset_idempotency_window", req.Spec.Workload.Kind)
				o.Metrics.IncStatefulSetControlledAction(req.Spec.Workload.Kind, "restart", "blocked", "none")
			}
			req.Status.Phase = ksv1alpha1.PhaseBlocked
			req.Status.LastError = "statefulset action already executed in current idempotency window"
			req.Status.BlockReasonCode = "statefulset_idempotency_window"
			req.Status.LastAction = "manual-intervention"
			req.Status.LastEventReason = "statefulset-idempotency-blocked"
			req.Status.ShadowAction = "would execute controlled statefulset action but blocked by idempotency window"
			o.emitRuntimeEvent(req, "Warning", "StatefulSetIdempotencyBlocked", req.Status.LastError)
			o.writeAudit(req, "blocked", req.Status.LastError)
			return errors.New(req.Status.LastError)
		}
		if err := o.Adapter.ValidateStatefulSetEvidence(ctx, req.Spec.Workload.Namespace, req.Spec.Workload.Name); err != nil {
			if o.Metrics != nil {
				o.Metrics.IncFailures()
				o.Metrics.IncReadOnlyBlocks("statefulset_evidence_missing", req.Spec.Workload.Kind)
				o.Metrics.IncStatefulSetControlledAction(req.Spec.Workload.Kind, "restart", "blocked", "none")
			}
			req.Status.Phase = ksv1alpha1.PhaseBlocked
			req.Status.LastError = "statefulset runtime evidence check failed; manual intervention required"
			req.Status.BlockReasonCode = "statefulset_evidence_missing"
			req.Status.LastAction = "manual-intervention"
			req.Status.LastEventReason = "statefulset-evidence-missing"
			req.Status.StatefulSetFailureReason = err.Error()
			req.Status.ShadowAction = "would execute controlled statefulset action but runtime evidence check failed"
			o.emitRuntimeEvent(req, "Warning", "StatefulSetEvidenceMissing", err.Error())
			o.writeAudit(req, "blocked", err.Error())
			return errors.New(req.Status.LastError)
		}
		snap, err := o.createSnapshot(ctx, req, "statefulset-l1")
		if err != nil {
			if o.Metrics != nil {
				o.Metrics.IncFailures()
				o.Metrics.IncSnapshotCreateFailure()
			}
			req.Status.Phase = ksv1alpha1.PhaseBlocked
			req.Status.LastError = err.Error()
			req.Status.BlockReasonCode = "snapshot_failed"
			req.Status.LastEventReason = "snapshot-failed"
			req.Status.SnapshotFailureReason = err.Error()
			o.emitRuntimeEvent(req, "Warning", "SnapshotCreateFailed", err.Error())
			o.writeAudit(req, "blocked", "statefulset snapshot creation failed")
			return err
		}
		o.recordActionAttempt(req.Spec.Workload.Namespace+"/"+req.Spec.Workload.Name+"/l1", o.Now(), req.Spec.IdempotencyWindowMinutes)
		if err := o.Adapter.ExecuteStatefulSetControlledAction(ctx, req.Spec.Workload.Namespace, req.Spec.Workload.Name, "restart"); err != nil {
			req.Status.Phase = ksv1alpha1.PhaseL2
			req.Status.StatefulSetL2Decision = "entered-l2-after-l1-failure"
			req.Status.StatefulSetFailureReason = err.Error()
			req.Status.LastEventReason = "statefulset-l1-failed"
			req.Status.LastAction = "statefulset-controlled-restart"
			if o.Metrics != nil {
				o.Metrics.IncFailures()
				o.Metrics.IncStatefulSetControlledAction(req.Spec.Workload.Kind, "restart", "failed", req.Status.StatefulSetFreezeState)
			}
			o.emitRuntimeEvent(req, "Warning", "StatefulSetL1Failed", err.Error())
			return o.processStatefulSetL2(ctx, req, runtimeInput, snap)
		}
		req.Status.Phase = ksv1alpha1.PhaseCompleted
		req.Status.LastAction = "statefulset-controlled-restart"
		req.Status.LastEventReason = "statefulset-controlled-action-succeeded"
		req.Status.LastEvidenceStatus = "statefulset-controlled-action-succeeded"
		req.Status.StatefulSetFreezeState = "none"
		req.Status.StatefulSetFreezeUntil = ""
		req.Status.StatefulSetFailureReason = ""
		req.Status.ObservedGeneration = req.Generation
		if o.Metrics != nil {
			o.Metrics.IncSuccess()
			o.Metrics.IncStatefulSetControlledAction(req.Spec.Workload.Kind, "restart", "executed", "none")
		}
		o.recordActionAttempt(req.Spec.Workload.Namespace+"/"+req.Spec.Workload.Name, o.Now(), req.Spec.RateLimit.WindowMinutes)
		o.emitRuntimeEvent(req, "Normal", "StatefulSetControlledActionSucceeded", "statefulset controlled restart executed")
		o.writeAudit(req, "success", "statefulset controlled restart executed")
		return nil
	}

	if blocked, reason := o.deploymentReleaseGateBlocked(req); blocked {
		req.Status.Phase = ksv1alpha1.PhaseBlocked
		req.Status.LastError = reason
		req.Status.BlockReasonCode = "deployment_release_gate"
		req.Status.LastAction = "manual-intervention"
		req.Status.LastEventReason = "deployment-release-gate-blocked"
		req.Status.NextRecommendation = "switch to conservative mode and adjust deployment tiered thresholds"
		if o.Metrics != nil {
			o.Metrics.IncFailures()
			o.Metrics.IncDeploymentL1Result("blocked")
			o.Metrics.IncDeploymentStageBlock("release_gate")
		}
		o.emitRuntimeEvent(req, "Warning", "DeploymentReleaseGateBlocked", reason)
		o.writeAudit(req, "blocked", reason)
		return errors.New(reason)
	}

	if o.actionsInWindow(req.Spec.Workload.Namespace+"/"+req.Spec.Workload.Name+"/l1", o.Now(), req.Spec.IdempotencyWindowMinutes) > 0 {
		req.Status.Phase = ksv1alpha1.PhaseBlocked
		req.Status.LastError = "deployment l1 action already executed in current idempotency window"
		req.Status.BlockReasonCode = "deployment_l1_idempotency_window"
		req.Status.LastAction = "manual-intervention"
		req.Status.LastEventReason = "deployment-l1-idempotency-blocked"
		req.Status.NextRecommendation = "wait for idempotency window or manually intervene"
		if o.Metrics != nil {
			o.Metrics.IncFailures()
			o.Metrics.IncReadOnlyBlocks("deployment_l1_idempotency_window", req.Spec.Workload.Kind)
			o.Metrics.IncDeploymentL1Result("blocked")
			o.Metrics.IncDeploymentStageBlock("idempotency_window")
		}
		o.emitRuntimeEvent(req, "Warning", "DeploymentL1IdempotencyBlocked", req.Status.LastError)
		o.writeAudit(req, "blocked", req.Status.LastError)
		return errors.New(req.Status.LastError)
	}

	snap, err := o.createSnapshot(ctx, req, "deployment-l1")
	if err != nil {
		logger.Error(err, "snapshot creation failed")
		if o.Metrics != nil {
			o.Metrics.IncFailures()
			o.Metrics.IncSnapshotCreateFailure()
			o.Metrics.IncDeploymentL1Result("failed")
		}
		req.Status.Phase = ksv1alpha1.PhaseBlocked
		req.Status.LastError = err.Error()
		req.Status.SnapshotFailureReason = err.Error()
		req.Status.LastEventReason = "snapshot-failed"
		req.Status.DeploymentL2Decision = "l1-snapshot-failed"
		o.emitRuntimeEvent(req, "Warning", "SnapshotCreateFailed", err.Error())
		return err
	}

	req.Status.Phase = ksv1alpha1.PhaseL1
	req.Status.LastAction = "deployment-l1-rollout-restart"
	req.Status.LastEventReason = "deployment-l1-started"
	if err := o.Adapter.ExecuteDeploymentControlledAction(ctx, req.Spec.Workload.Namespace, req.Spec.Workload.Name, "rollout-restart"); err != nil {
		req.Status.Phase = ksv1alpha1.PhaseL2
		req.Status.DeploymentL2Decision = "entered-l2-after-l1-failure"
		req.Status.DeploymentL2Result = "pending"
		req.Status.LastError = fmt.Sprintf("deployment l1 action failed: %v", err)
		req.Status.LastEventReason = "deployment-l1-failed"
		req.Status.NextRecommendation = "evaluate deployment L2 healthy rollback candidate"
		if breaker != nil {
			breaker.RecordFailure(req.Spec.Workload.Namespace+"/"+req.Spec.Workload.Name, o.Now())
			if allow, reason := breaker.Allow(req.Spec.Workload.Namespace+"/"+req.Spec.Workload.Name, o.Now()); !allow {
				state := breaker.Status(req.Spec.Workload.Namespace + "/" + req.Spec.Workload.Name)
				req.Status.Phase = ksv1alpha1.PhaseBlocked
				req.Status.BlockReasonCode = "circuit_breaker_open"
				req.Status.LastError = reason
				req.Status.LastGateDecision = reason
				req.Status.DeploymentL2Decision = "blocked-by-circuit-breaker"
				req.Status.DeploymentL2Result = "fallback"
				req.Status.LastEventReason = "deployment-l2-circuit-breaker-open"
				req.Status.CircuitBreaker.ObjectOpen = state.OpenReason == "object breaker open"
				req.Status.CircuitBreaker.DomainOpen = state.OpenReason == "domain breaker open"
				req.Status.CircuitBreaker.CurrentObjectFailures = state.CurrentObjectFailures
				req.Status.CircuitBreaker.CurrentDomainFailures = state.CurrentDomainFailures
				req.Status.CircuitBreaker.RecoveryAt = state.RecoveryAt
				req.Status.CircuitBreaker.OpenReason = fmt.Sprintf("%s (objectThreshold=%d, domainThreshold=%d)", state.OpenReason, req.Spec.CircuitBreaker.ObjectFailureThreshold, req.Spec.CircuitBreaker.DomainFailureThreshold)
				if o.Metrics != nil {
					o.Metrics.IncFailures()
					o.Metrics.IncCircuitBreaks()
					o.Metrics.IncDeploymentL1Result("failed")
					o.Metrics.IncDeploymentL2Result("fallback")
					o.Metrics.IncDeploymentStageBlock("circuit_breaker")
				}
				o.emitRuntimeEvent(req, "Warning", "CircuitBreakerOpen", reason)
				o.writeAudit(req, "blocked", req.Status.CircuitBreaker.OpenReason)
				return errors.New(reason)
			}
		}
		if o.Metrics != nil {
			o.Metrics.IncFailures()
			o.Metrics.IncDeploymentL1Result("failed")
		}
		o.emitRuntimeEvent(req, "Warning", "DeploymentL1Failed", err.Error())
		return o.processDeploymentL2(ctx, req, snap)
	}

	o.recordActionAttempt(req.Spec.Workload.Namespace+"/"+req.Spec.Workload.Name+"/l1", o.Now(), req.Spec.IdempotencyWindowMinutes)
	o.recordActionAttempt(req.Spec.Workload.Namespace+"/"+req.Spec.Workload.Name, o.Now(), req.Spec.RateLimit.WindowMinutes)
	req.Status.Phase = ksv1alpha1.PhaseCompleted
	req.Status.LastAction = "deployment-l1-rollout-restart"
	req.Status.LastEventReason = "deployment-l1-succeeded"
	req.Status.LastEvidenceStatus = "deployment-l1-succeeded"
	req.Status.DeploymentL2Decision = "not-required-l1-succeeded"
	req.Status.DeploymentL2Result = "skipped"
	req.Status.LastError = ""
	req.Status.NextRecommendation = "continue observing post-l1 stability"
	req.Status.ObservedGeneration = req.Generation
	if o.Metrics != nil {
		o.Metrics.IncSuccess()
		o.Metrics.IncDeploymentL1Result("success")
	}
	o.writeAudit(req, "success", "deployment l1 action executed")
	o.emitRuntimeEvent(req, "Normal", "DeploymentL1Succeeded", "deployment l1 rollout restart executed")
	return nil
}

func (o *Orchestrator) advanceSoakWindow(req *ksv1alpha1.HealingRequest, duration time.Duration, minSamples int) (bool, bool) {
	now := o.Now()
	if req.Status.PendingSince == "" {
		req.Status.PendingSince = now.Format(time.RFC3339)
		req.Status.StableSampleCount = 1
		req.Status.Phase = ksv1alpha1.PhasePendingVerify
		req.Status.LastGateDecision = fmt.Sprintf("pending verify (soak=%s,minSamples=%d)", duration.String(), minSamples)
		return true, false
	}
	pendingAt, err := time.Parse(time.RFC3339, req.Status.PendingSince)
	if err != nil {
		req.Status.PendingSince = now.Format(time.RFC3339)
		req.Status.StableSampleCount = 1
		req.Status.Phase = ksv1alpha1.PhasePendingVerify
		req.Status.LastGateDecision = fmt.Sprintf("pending verify (soak=%s,minSamples=%d)", duration.String(), minSamples)
		return true, false
	}
	req.Status.StableSampleCount++
	if now.Sub(pendingAt) < duration || req.Status.StableSampleCount < minSamples {
		req.Status.Phase = ksv1alpha1.PhasePendingVerify
		req.Status.LastGateDecision = fmt.Sprintf("pending verify (soak=%s,stableSamples=%d/%d)", duration.String(), req.Status.StableSampleCount, minSamples)
		return false, false
	}
	req.Status.PendingSince = ""
	req.Status.StableSampleCount = 0
	req.Status.LastEvidenceStatus = "soak-window-passed"
	return false, true
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
		ID:           req.Status.CorrelationKey,
		Trigger:      "alertmanager",
		Target:       req.Spec.Workload.Namespace + "/" + req.Spec.Workload.Name,
		WorkloadKind: req.Spec.Workload.Kind,
		ActionType:   req.Status.LastAction,
		Phase:        string(req.Status.Phase),
		Decision:     req.Status.LastGateDecision,
		FreezeState:  req.Status.StatefulSetFreezeState,
		SnapshotID:   req.Status.LastSnapshotID,
		BeforeState:  req.Status.LastEventReason,
		AfterState:   afterState,
		Result:       result,
		CreatedAt:    o.Now(),
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
		SnapshotID:     req.Status.LastSnapshotID,
		Type:           eventType,
		CreatedAt:      o.Now(),
	})
	if o.K8sEventRecorder != nil {
		o.K8sEventRecorder.Eventf(req, eventType, reason, "%s", message)
	}
}

func isResolvedAlert(req *ksv1alpha1.HealingRequest) bool {
	if req.Annotations == nil {
		return false
	}
	status := strings.ToLower(strings.TrimSpace(req.Annotations["kube-sentinel.io/alert-status"]))
	return status == "resolved"
}

func hasAlertMetadata(req *ksv1alpha1.HealingRequest) bool {
	if req.Annotations == nil {
		return false
	}
	status := strings.TrimSpace(req.Annotations["kube-sentinel.io/alert-status"])
	return status != ""
}

func isCriticalWorkload(req *ksv1alpha1.HealingRequest) bool {
	if req.Annotations == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(req.Annotations["kube-sentinel.io/criticality"]), "high")
}

func soakProfileFor(req *ksv1alpha1.HealingRequest) (time.Duration, int) {
	category := ""
	severity := ""
	if req.Annotations != nil {
		category = req.Annotations["kube-sentinel.io/alert-category"]
		severity = req.Annotations["kube-sentinel.io/alert-severity"]
	}
	for _, profile := range req.Spec.SoakTimePolicies {
		if strings.EqualFold(profile.Category, category) && strings.EqualFold(profile.Severity, severity) {
			return time.Duration(profile.DurationSec) * time.Second, profile.MinSamples
		}
	}
	return 120 * time.Second, 3
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func workloadCapabilityForRequest(req *ksv1alpha1.HealingRequest) string {
	if req.Spec.Workload.Kind == "StatefulSet" {
		if req.Spec.StatefulSetPolicy.Enabled && !req.Spec.StatefulSetPolicy.ReadOnlyOnly && req.Spec.StatefulSetPolicy.ControlledActionsEnabled {
			return "conditional-writable"
		}
		return "read-only"
	}
	if req.Spec.Workload.Kind == "Deployment" {
		return "writable"
	}
	return "unsupported"
}

func authorizeStatefulSet(req *ksv1alpha1.HealingRequest, runtimeInput RuntimeInput) (bool, string) {
	policy := req.Spec.StatefulSetPolicy
	if !policy.Enabled {
		return false, "statefulset policy is disabled"
	}
	if policy.ReadOnlyOnly || !policy.ControlledActionsEnabled {
		return false, "statefulset policy is read-only"
	}
	if !isAllowedNamespace(req.Spec.Workload.Namespace, policy.AllowedNamespaces) {
		return false, "namespace is not in statefulset allowedNamespaces"
	}
	if req.Annotations == nil || !strings.EqualFold(strings.TrimSpace(req.Annotations[policy.ApprovalAnnotation]), "true") {
		return false, "approval annotation is missing or not true"
	}
	if policy.RequireEvidence {
		if req.Status.LastEvidenceStatus == "" || req.Status.LastEvidenceStatus == "insufficient-evidence" {
			return false, "runtime evidence is insufficient"
		}
		if runtimeInput.ClusterPods < 1 {
			return false, "runtime evidence is incomplete"
		}
	}
	return true, "authorized"
}

func isAllowedNamespace(namespace string, allowed []string) bool {
	for _, candidate := range allowed {
		if strings.TrimSpace(candidate) == namespace {
			return true
		}
	}
	return false
}

func isStatefulSetFrozen(req *ksv1alpha1.HealingRequest, now time.Time) (bool, string) {
	if req.Status.StatefulSetFreezeState != "frozen" || req.Status.StatefulSetFreezeUntil == "" {
		return false, ""
	}
	freezeUntil, err := time.Parse(time.RFC3339, req.Status.StatefulSetFreezeUntil)
	if err != nil {
		return false, ""
	}
	if now.Before(freezeUntil) {
		remaining := freezeUntil.Sub(now).Round(time.Second)
		return true, fmt.Sprintf("statefulset is in freeze window; remaining=%s", remaining.String())
	}
	req.Status.StatefulSetFreezeState = "none"
	req.Status.StatefulSetFreezeUntil = ""
	return false, ""
}

func selectDeploymentL2Candidate(revisions []RevisionRecord, windowMinutes int, now time.Time) (RevisionRecord, error) {
	windowStart := now.Add(-time.Duration(windowMinutes) * time.Minute).Unix()
	for _, candidate := range revisions {
		if candidate.Healthy && candidate.UnixTime >= windowStart {
			return candidate, nil
		}
	}
	return RevisionRecord{}, fmt.Errorf("no healthy revision within %d minute candidate window", windowMinutes)
}

func (o *Orchestrator) deploymentReleaseGateBlocked(req *ksv1alpha1.HealingRequest) (bool, string) {
	if o.Metrics == nil {
		return false, ""
	}
	l1Rate, l2Rate, l3Rate, blockRate := o.Metrics.DeploymentTieredRates()
	policy := req.Spec.DeploymentPolicy
	if l1Rate < float64(policy.L1SuccessRateMinPercent) {
		return true, fmt.Sprintf("deployment release gate blocked: l1 success rate %.1f%% < %d%%", l1Rate, policy.L1SuccessRateMinPercent)
	}
	if l2Rate < float64(policy.L2SuccessRateMinPercent) {
		return true, fmt.Sprintf("deployment release gate blocked: l2 success rate %.1f%% < %d%%", l2Rate, policy.L2SuccessRateMinPercent)
	}
	if l3Rate > float64(policy.L3DegradeRateMaxPercent) {
		return true, fmt.Sprintf("deployment release gate blocked: l3 degrade rate %.1f%% > %d%%", l3Rate, policy.L3DegradeRateMaxPercent)
	}
	if blockRate > float64(policy.BlockRateMaxPercent) {
		return true, fmt.Sprintf("deployment release gate blocked: stage block rate %.1f%% > %d%%", blockRate, policy.BlockRateMaxPercent)
	}
	return false, ""
}

func (o *Orchestrator) processDeploymentL2(ctx context.Context, req *ksv1alpha1.HealingRequest, snapshot Snapshot) error {
	if o.actionsInWindow(req.Spec.Workload.Namespace+"/"+req.Spec.Workload.Name+"/l2", o.Now(), req.Spec.IdempotencyWindowMinutes) > 0 {
		req.Status.Phase = ksv1alpha1.PhaseBlocked
		req.Status.DeploymentL2Result = "fallback"
		req.Status.DeploymentL2Decision = "blocked-by-idempotency-window"
		req.Status.LastError = "deployment l2 rollback blocked by idempotency window"
		req.Status.BlockReasonCode = "deployment_l2_idempotency_window"
		req.Status.LastAction = "manual-intervention"
		req.Status.LastEventReason = "deployment-l2-idempotency-blocked"
		req.Status.NextRecommendation = "wait for idempotency window or manually intervene"
		if o.Metrics != nil {
			o.Metrics.IncFailures()
			o.Metrics.IncReadOnlyBlocks("deployment_l2_idempotency_window", req.Spec.Workload.Kind)
			o.Metrics.IncDeploymentL2Result("fallback")
			o.Metrics.IncDeploymentStageBlock("idempotency_window")
		}
		o.emitRuntimeEvent(req, "Warning", "DeploymentL2IdempotencyBlocked", req.Status.LastError)
		o.writeAudit(req, "blocked", req.Status.LastError)
		return errors.New(req.Status.LastError)
	}

	revisions, err := o.Adapter.ListRevisions(ctx, req.Spec.Workload.Namespace, req.Spec.Workload.Name)
	if err != nil {
		req.Status.Phase = ksv1alpha1.PhaseL3
		req.Status.DeploymentL2Result = "degraded"
		req.Status.DeploymentL2Decision = "revision-list-failed"
		req.Status.LastEvidenceStatus = "insufficient-evidence"
		req.Status.LastAction = "manual-intervention"
		req.Status.LastError = fmt.Sprintf("deployment l2 revision list failed: %v", err)
		req.Status.LastEventReason = "deployment-l2-revision-list-failed"
		req.Status.NextRecommendation = "verify deployment revision history"
		if o.Metrics != nil {
			o.Metrics.IncFailures()
			o.Metrics.IncDeploymentL2Result("degraded")
		}
		o.emitRuntimeEvent(req, "Warning", "DeploymentL2Degraded", req.Status.LastError)
		o.writeAudit(req, "fallback", req.Status.LastError)
		return nil
	}
	latest, err := selectDeploymentL2Candidate(revisions, req.Spec.DeploymentPolicy.L2CandidateWindowMinutes, o.Now())
	if err != nil {
		req.Status.Phase = ksv1alpha1.PhaseL3
		req.Status.DeploymentL2Result = "degraded"
		req.Status.DeploymentL2Decision = "no-healthy-candidate"
		req.Status.LastEvidenceStatus = "insufficient-evidence"
		req.Status.LastAction = "manual-intervention"
		req.Status.LastError = "deployment no healthy revision available in candidate window; degraded to manual intervention"
		req.Status.LastEventReason = "deployment-l2-no-candidate"
		req.Status.NextRecommendation = "inspect healthy revision evidence and alert history"
		if o.Metrics != nil {
			o.Metrics.IncDeploymentL2Result("degraded")
		}
		o.emitRuntimeEvent(req, "Warning", "DeploymentL2Degraded", req.Status.LastError)
		o.writeAudit(req, "fallback", req.Status.LastError)
		return nil
	}
	req.Status.DeploymentL2Candidate = latest.Revision
	req.Status.LastEvidenceStatus = "deployment-l2-candidate-selected"
	if err := o.Adapter.ValidateRevisionDependencies(ctx, req.Spec.Workload.Namespace, req.Spec.Workload.Name, latest.Revision); err != nil {
		req.Status.Phase = ksv1alpha1.PhaseL3
		req.Status.DeploymentL2Result = "degraded"
		req.Status.DeploymentL2Decision = "dependency-validation-failed"
		req.Status.LastEvidenceStatus = "insufficient-evidence"
		req.Status.LastAction = "manual-intervention"
		req.Status.LastError = "deployment revision dependencies unavailable; degraded to manual intervention"
		req.Status.LastEventReason = "deployment-l2-dependency-missing"
		req.Status.NextRecommendation = "restore missing dependencies and retry"
		if o.Metrics != nil {
			o.Metrics.IncFailures()
			o.Metrics.IncReadOnlyBlocks("deployment_l2_dependency", req.Spec.Workload.Kind)
			o.Metrics.IncDeploymentL2Result("degraded")
		}
		o.emitRuntimeEvent(req, "Warning", "DeploymentL2Degraded", req.Status.LastError)
		o.writeAudit(req, "fallback", req.Status.LastError)
		return nil
	}
	o.recordActionAttempt(req.Spec.Workload.Namespace+"/"+req.Spec.Workload.Name+"/l2", o.Now(), req.Spec.IdempotencyWindowMinutes)
	if err := o.Adapter.RollbackToRevision(ctx, req.Spec.Workload.Namespace, req.Spec.Workload.Name, latest.Revision); err != nil {
		restoreErr := o.restoreSnapshot(ctx, req, snapshot)
		req.Status.Phase = ksv1alpha1.PhaseBlocked
		req.Status.DeploymentL2Result = "fallback"
		req.Status.DeploymentL2Decision = "rollback-failed"
		req.Status.LastError = "deployment l2 rollback failed; fallback to read-only"
		req.Status.BlockReasonCode = "deployment_l2_rollback_failed"
		req.Status.LastAction = "manual-intervention"
		req.Status.LastEventReason = "deployment-l2-rollback-failed"
		req.Status.StatefulSetFailureReason = err.Error()
		if restoreErr != nil {
			req.Status.StatefulSetFailureReason = fmt.Sprintf("rollback failed: %v; restore failed: %v", err, restoreErr)
		}
		req.Status.NextRecommendation = "manual intervention required before retrying automated action"
		if o.Metrics != nil {
			o.Metrics.IncFailures()
			o.Metrics.IncReadOnlyBlocks("deployment_l2_rollback_failed", req.Spec.Workload.Kind)
			o.Metrics.IncDeploymentL2Result("fallback")
			o.Metrics.IncDeploymentStageBlock("rollback_failed")
		}
		o.emitRuntimeEvent(req, "Warning", "DeploymentL2RollbackFailed", err.Error())
		o.writeAudit(req, "failed", "deployment l2 rollback failed and restored snapshot")
		return err
	}

	req.Status.Phase = ksv1alpha1.PhaseCompleted
	req.Status.DeploymentL2Result = "success"
	req.Status.DeploymentL2Decision = "rollback-succeeded"
	req.Status.LastAction = "deployment-l2-rollback-to-healthy"
	req.Status.LastHealthyRevision = latest.Revision
	req.Status.LastEventReason = "deployment-l2-rollback-succeeded"
	req.Status.LastEvidenceStatus = "deployment-l2-rollback-succeeded"
	req.Status.NextRecommendation = "continue observing post-rollback stability"
	req.Status.ObservedGeneration = req.Generation
	if o.Metrics != nil {
		o.Metrics.IncRollbacks()
		o.Metrics.IncSuccess()
		o.Metrics.IncDeploymentL2Result("success")
	}
	o.recordActionAttempt(req.Spec.Workload.Namespace+"/"+req.Spec.Workload.Name, o.Now(), req.Spec.RateLimit.WindowMinutes)
	o.emitRuntimeEvent(req, "Normal", "DeploymentL2RollbackSucceeded", "deployment l2 rollback executed")
	o.writeAudit(req, "success", "deployment l2 rollback executed")
	return nil
}

func (o *Orchestrator) processStatefulSetL2(ctx context.Context, req *ksv1alpha1.HealingRequest, runtimeInput RuntimeInput, snapshot Snapshot) error {
	if !req.Spec.StatefulSetPolicy.L2RollbackEnabled {
		req.Status.Phase = ksv1alpha1.PhaseL3
		req.Status.StatefulSetL2Result = "degraded"
		req.Status.StatefulSetL2Decision = "l2-disabled"
		req.Status.LastEvidenceStatus = "insufficient-evidence"
		req.Status.LastAction = "manual-intervention"
		req.Status.LastError = "statefulset l2 rollback is disabled; degraded to manual intervention"
		req.Status.LastEventReason = "statefulset-l2-disabled"
		req.Status.NextRecommendation = "enable statefulSetPolicy.l2RollbackEnabled to allow L2 rollback"
		if o.Metrics != nil {
			o.Metrics.IncStatefulSetL2Result("degraded")
		}
		o.emitRuntimeEvent(req, "Warning", "StatefulSetL2Degraded", req.Status.LastError)
		o.writeAudit(req, "fallback", req.Status.LastError)
		return nil
	}
	if o.actionsInWindow(req.Spec.Workload.Namespace+"/"+req.Spec.Workload.Name+"/l2", o.Now(), req.Spec.IdempotencyWindowMinutes) > 0 {
		req.Status.Phase = ksv1alpha1.PhaseBlocked
		req.Status.StatefulSetL2Result = "fallback"
		req.Status.StatefulSetL2Decision = "blocked-by-idempotency-window"
		req.Status.LastError = "statefulset l2 rollback blocked by idempotency window"
		req.Status.BlockReasonCode = "statefulset_l2_idempotency_window"
		req.Status.LastAction = "manual-intervention"
		req.Status.LastEventReason = "statefulset-l2-idempotency-blocked"
		req.Status.NextRecommendation = "wait for idempotency window or manually intervene"
		if o.Metrics != nil {
			o.Metrics.IncFailures()
			o.Metrics.IncReadOnlyBlocks("statefulset_l2_idempotency_window", req.Spec.Workload.Kind)
			o.Metrics.IncStatefulSetControlledAction(req.Spec.Workload.Kind, "l2_rollback", "blocked", req.Status.StatefulSetFreezeState)
			o.Metrics.IncStatefulSetL2Result("fallback")
		}
		o.emitRuntimeEvent(req, "Warning", "StatefulSetL2IdempotencyBlocked", req.Status.LastError)
		o.writeAudit(req, "blocked", req.Status.LastError)
		return errors.New(req.Status.LastError)
	}
	if req.Spec.StatefulSetPolicy.RequireEvidence && runtimeInput.ClusterPods < 1 {
		req.Status.Phase = ksv1alpha1.PhaseL3
		req.Status.StatefulSetL2Result = "degraded"
		req.Status.StatefulSetL2Decision = "insufficient-runtime-evidence"
		req.Status.LastEvidenceStatus = "insufficient-evidence"
		req.Status.LastAction = "manual-intervention"
		req.Status.LastError = "statefulset runtime evidence is insufficient for L2 rollback"
		req.Status.LastEventReason = "statefulset-l2-evidence-insufficient"
		req.Status.NextRecommendation = "stabilize workload evidence and retry"
		if o.Metrics != nil {
			o.Metrics.IncStatefulSetL2Result("degraded")
		}
		o.emitRuntimeEvent(req, "Warning", "StatefulSetL2Degraded", req.Status.LastError)
		o.writeAudit(req, "fallback", req.Status.LastError)
		return nil
	}
	revisions, err := o.Adapter.ListRevisions(ctx, req.Spec.Workload.Namespace, req.Spec.Workload.Name)
	if err != nil {
		req.Status.Phase = ksv1alpha1.PhaseL3
		req.Status.StatefulSetL2Result = "degraded"
		req.Status.StatefulSetL2Decision = "revision-list-failed"
		req.Status.LastEvidenceStatus = "insufficient-evidence"
		req.Status.LastAction = "manual-intervention"
		req.Status.LastError = fmt.Sprintf("statefulset l2 revision list failed: %v", err)
		req.Status.LastEventReason = "statefulset-l2-revision-list-failed"
		req.Status.NextRecommendation = "verify statefulset revision history"
		if o.Metrics != nil {
			o.Metrics.IncFailures()
			o.Metrics.IncStatefulSetL2Result("degraded")
		}
		o.emitRuntimeEvent(req, "Warning", "StatefulSetL2Degraded", req.Status.LastError)
		o.writeAudit(req, "fallback", req.Status.LastError)
		return nil
	}
	latest, err := SelectLatestHealthyRevision(revisions)
	if err != nil {
		req.Status.Phase = ksv1alpha1.PhaseL3
		req.Status.StatefulSetL2Result = "degraded"
		req.Status.StatefulSetL2Decision = "no-healthy-candidate"
		req.Status.LastEvidenceStatus = "insufficient-evidence"
		req.Status.LastAction = "manual-intervention"
		req.Status.LastError = "statefulset no healthy revision available; degraded to manual intervention"
		req.Status.LastEventReason = "statefulset-l2-no-candidate"
		req.Status.NextRecommendation = "inspect healthy revision evidence and alert history"
		if o.Metrics != nil {
			o.Metrics.IncStatefulSetL2Result("degraded")
		}
		o.emitRuntimeEvent(req, "Warning", "StatefulSetL2Degraded", req.Status.LastError)
		o.writeAudit(req, "fallback", req.Status.LastError)
		return nil
	}
	req.Status.StatefulSetL2Candidate = latest.Revision
	req.Status.LastEvidenceStatus = "statefulset-l2-candidate-selected"
	if err := o.Adapter.ValidateRevisionDependencies(ctx, req.Spec.Workload.Namespace, req.Spec.Workload.Name, latest.Revision); err != nil {
		req.Status.Phase = ksv1alpha1.PhaseL3
		req.Status.StatefulSetL2Result = "degraded"
		req.Status.StatefulSetL2Decision = "dependency-validation-failed"
		req.Status.LastEvidenceStatus = "insufficient-evidence"
		req.Status.LastAction = "manual-intervention"
		req.Status.LastError = "statefulset revision dependencies unavailable; degraded to manual intervention"
		req.Status.LastEventReason = "statefulset-l2-dependency-missing"
		req.Status.NextRecommendation = "restore missing dependencies and retry"
		if o.Metrics != nil {
			o.Metrics.IncFailures()
			o.Metrics.IncReadOnlyBlocks("statefulset_l2_dependency", req.Spec.Workload.Kind)
			o.Metrics.IncStatefulSetL2Result("degraded")
		}
		o.emitRuntimeEvent(req, "Warning", "StatefulSetL2Degraded", req.Status.LastError)
		o.writeAudit(req, "fallback", req.Status.LastError)
		return nil
	}
	o.recordActionAttempt(req.Spec.Workload.Namespace+"/"+req.Spec.Workload.Name+"/l2", o.Now(), req.Spec.IdempotencyWindowMinutes)
	if err := o.Adapter.RollbackToRevision(ctx, req.Spec.Workload.Namespace, req.Spec.Workload.Name, latest.Revision); err != nil {
		restoreErr := o.restoreSnapshot(ctx, req, snapshot)
		freezeUntil := o.Now().Add(time.Duration(req.Spec.StatefulSetPolicy.FreezeWindowMinutes) * time.Minute)
		req.Status.Phase = ksv1alpha1.PhaseBlocked
		req.Status.StatefulSetL2Result = "fallback"
		req.Status.StatefulSetL2Decision = "rollback-failed"
		req.Status.LastError = "statefulset l2 rollback failed; fallback to read-only"
		req.Status.BlockReasonCode = "statefulset_l2_rollback_failed"
		req.Status.LastAction = "manual-intervention"
		req.Status.LastEventReason = "statefulset-l2-rollback-failed"
		req.Status.StatefulSetFreezeState = "frozen"
		req.Status.StatefulSetFreezeUntil = freezeUntil.Format(time.RFC3339)
		req.Status.StatefulSetFailureReason = err.Error()
		if restoreErr != nil {
			req.Status.StatefulSetFailureReason = fmt.Sprintf("rollback failed: %v; restore failed: %v", err, restoreErr)
		}
		req.Status.NextRecommendation = "manual intervention required before unlocking freeze"
		if o.Metrics != nil {
			o.Metrics.IncFailures()
			o.Metrics.IncReadOnlyBlocks("statefulset_l2_rollback_failed", req.Spec.Workload.Kind)
			o.Metrics.IncStatefulSetControlledAction(req.Spec.Workload.Kind, "l2_rollback", "fallback", "frozen")
			o.Metrics.IncStatefulSetFreezeTriggers()
			o.Metrics.IncStatefulSetL2Result("fallback")
		}
		o.emitRuntimeEvent(req, "Warning", "StatefulSetL2RollbackFailed", err.Error())
		o.writeAudit(req, "failed", "statefulset l2 rollback failed and restored snapshot")
		return err
	}
	req.Status.Phase = ksv1alpha1.PhaseCompleted
	req.Status.StatefulSetL2Result = "success"
	req.Status.StatefulSetL2Decision = "rollback-succeeded"
	req.Status.LastAction = "statefulset-l2-rollback-to-healthy"
	req.Status.LastHealthyRevision = latest.Revision
	req.Status.LastEventReason = "statefulset-l2-rollback-succeeded"
	req.Status.LastEvidenceStatus = "statefulset-l2-rollback-succeeded"
	req.Status.StatefulSetFailureReason = ""
	req.Status.StatefulSetFreezeState = "none"
	req.Status.StatefulSetFreezeUntil = ""
	req.Status.NextRecommendation = "continue observing post-rollback stability"
	req.Status.ObservedGeneration = req.Generation
	if o.Metrics != nil {
		o.Metrics.IncRollbacks()
		o.Metrics.IncSuccess()
		o.Metrics.IncStatefulSetControlledAction(req.Spec.Workload.Kind, "l2_rollback", "executed", "none")
		o.Metrics.IncStatefulSetL2Result("success")
	}
	o.recordActionAttempt(req.Spec.Workload.Namespace+"/"+req.Spec.Workload.Name, o.Now(), req.Spec.RateLimit.WindowMinutes)
	o.emitRuntimeEvent(req, "Normal", "StatefulSetL2RollbackSucceeded", "statefulset l2 rollback executed")
	o.writeAudit(req, "success", "statefulset l2 rollback executed")
	return nil
}

func (o *Orchestrator) createSnapshot(ctx context.Context, req *ksv1alpha1.HealingRequest, phase string) (Snapshot, error) {
	if o.Snapshotter == nil {
		return Snapshot{}, fmt.Errorf("snapshotter is required")
	}
	if pruned, err := o.Snapshotter.Prune(
		ctx,
		req.Spec.Workload.Namespace,
		req.Spec.Workload.Name,
		req.Spec.SnapshotPolicy.RetentionMinutes,
		req.Spec.SnapshotPolicy.MaxSnapshotsPerWorkload,
	); err == nil {
		if o.Metrics != nil && pruned > 0 {
			o.Metrics.AddSnapshotPruned(pruned)
		}
	}
	windowSeconds := int64(maxInt(req.Spec.IdempotencyWindowMinutes, 1) * 60)
	bucket := o.Now().Unix() / windowSeconds
	snapshot, err := o.Snapshotter.Create(ctx, req.Spec.Workload.Namespace, req.Spec.Workload.Name, SnapshotOptions{
		WorkloadKind:      req.Spec.Workload.Kind,
		Phase:             phase,
		IdempotencyKey:    fmt.Sprintf("%s/%s/%s/%d", req.Spec.Workload.Namespace, req.Spec.Workload.Name, phase, bucket),
		RetentionMinutes:  req.Spec.SnapshotPolicy.RetentionMinutes,
		MaxSnapshotsCount: req.Spec.SnapshotPolicy.MaxSnapshotsPerWorkload,
	})
	if err != nil {
		if o.Metrics != nil {
			o.Metrics.IncSnapshotCapacityBlock()
		}
		return Snapshot{}, err
	}
	req.Status.LastSnapshotID = snapshot.ID
	req.Status.SnapshotRestoreResult = "pending"
	req.Status.SnapshotFailureReason = ""
	if snapshots, listErr := o.Snapshotter.List(ctx, req.Spec.Workload.Namespace, req.Spec.Workload.Name); listErr == nil {
		req.Status.SnapshotActiveCount = len(snapshots)
		if o.Metrics != nil {
			o.Metrics.SetSnapshotActive(len(snapshots))
		}
	}
	if o.Metrics != nil {
		o.Metrics.IncSnapshotCreateSuccess()
	}
	return snapshot, nil
}

func (o *Orchestrator) restoreSnapshot(ctx context.Context, req *ksv1alpha1.HealingRequest, snapshot Snapshot) error {
	if o.Snapshotter == nil {
		return fmt.Errorf("snapshotter is required")
	}
	startedAt := o.Now()
	err := o.Snapshotter.Restore(ctx, snapshot)
	if o.Metrics != nil {
		o.Metrics.ObserveSnapshotRestoreDuration(o.Now().Sub(startedAt))
	}
	if err != nil {
		req.Status.SnapshotRestoreResult = "failed"
		req.Status.SnapshotFailureReason = err.Error()
		if o.Metrics != nil {
			o.Metrics.IncSnapshotRestoreFailure()
		}
		return err
	}
	req.Status.SnapshotRestoreResult = "success"
	req.Status.SnapshotFailureReason = ""
	if o.Metrics != nil {
		o.Metrics.IncSnapshotRestoreSuccess()
	}
	return nil
}
