package healing

import (
	"context"
	"fmt"

	ksv1alpha1 "github.com/yangyus8/kube-sentinel/api/v1alpha1"
)

type RuntimeInput struct {
	ActionsInWindow int
	AffectedPods    int
	ClusterPods     int
}

type RuntimeInputProvider interface {
	Build(ctx context.Context, req *ksv1alpha1.HealingRequest) (RuntimeInput, error)
}

type adapterRuntimeInputProvider struct {
	adapter WorkloadAdapter
}

func (p adapterRuntimeInputProvider) Build(ctx context.Context, req *ksv1alpha1.HealingRequest) (RuntimeInput, error) {
	if p.adapter == nil {
		return RuntimeInput{}, fmt.Errorf("workload adapter is required")
	}
	affectedPods, err := p.adapter.CountAffectedPods(ctx, req.Spec.Workload.Namespace, req.Spec.Workload.Name)
	if err != nil {
		return RuntimeInput{}, err
	}
	clusterPods, err := p.adapter.CountClusterPods(ctx, req.Spec.Workload.Namespace)
	if err != nil {
		return RuntimeInput{}, err
	}
	return RuntimeInput{
		ActionsInWindow: 0,
		AffectedPods:    affectedPods,
		ClusterPods:     clusterPods,
	}, nil
}
