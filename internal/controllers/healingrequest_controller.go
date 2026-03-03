package controllers

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ksv1alpha1 "github.com/yangyus8/kube-sentinel/api/v1alpha1"
	"github.com/yangyus8/kube-sentinel/internal/healing"
	"github.com/yangyus8/kube-sentinel/internal/observability"
	"github.com/yangyus8/kube-sentinel/internal/safety"
)

type HealingRequestReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	Orchestrator *healing.Orchestrator
}

func (r *HealingRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var resource ksv1alpha1.HealingRequest
	if err := r.Get(ctx, req.NamespacedName, &resource); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if r.Orchestrator == nil {
		r.Orchestrator = &healing.Orchestrator{
			Adapter:     healing.DeploymentAdapter{},
			Snapshotter: &healing.MemorySnapshotter{},
			Breaker:     safety.NewCircuitBreaker(3, 10, 10),
			Metrics:     &observability.Metrics{},
			AuditSink:   &observability.MemoryAuditSink{},
			EventSink:   &observability.MemoryEventSink{},
		}
	}
	if err := r.Orchestrator.Process(ctx, &resource); err != nil {
		_ = r.Status().Update(ctx, &resource)
		return ctrl.Result{}, nil
	}
	if err := r.Status().Update(ctx, &resource); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *HealingRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ksv1alpha1.HealingRequest{}).
		Complete(r)
}
