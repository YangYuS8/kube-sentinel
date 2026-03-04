package healing

import "context"

type RevisionRecord struct {
	Revision string
	UnixTime int64
	Healthy  bool
}

type WorkloadAdapter interface {
	Kind() string
	Supports(kind string) bool
	ListRevisions(ctx context.Context, namespace, name string) ([]RevisionRecord, error)
	RollbackToRevision(ctx context.Context, namespace, name, revision string) error
	ExecuteDeploymentControlledAction(ctx context.Context, namespace, name, actionType string) error
	ExecuteStatefulSetControlledAction(ctx context.Context, namespace, name, actionType string) error
	ValidateStatefulSetEvidence(ctx context.Context, namespace, name string) error
	ValidateRevisionDependencies(ctx context.Context, namespace, name, revision string) error
	CountAffectedPods(ctx context.Context, namespace, name string) (int, error)
	CountClusterPods(ctx context.Context, namespace string) (int, error)
	CountTotalWorkloads(ctx context.Context, namespace string) (int, error)
	CountUnhealthyWorkloads(ctx context.Context, namespace string) (int, error)
}
