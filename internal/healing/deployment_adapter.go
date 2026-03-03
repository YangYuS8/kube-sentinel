package healing

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const deploymentRevisionAnnotation = "deployment.kubernetes.io/revision"

type DeploymentAdapter struct {
	Client client.Client
}

func NewDeploymentAdapter(k8sClient client.Client) DeploymentAdapter {
	return DeploymentAdapter{Client: k8sClient}
}

func (DeploymentAdapter) Kind() string {
	return "Deployment"
}

func (DeploymentAdapter) Supports(kind string) bool {
	return kind == "Deployment"
}

func (d DeploymentAdapter) ListRevisions(ctx context.Context, namespace, name string) ([]RevisionRecord, error) {
	if d.Client == nil {
		return nil, fmt.Errorf("kubernetes client is required")
	}
	deployment := appsv1.Deployment{}
	if err := d.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &deployment); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("deployment %s/%s not found", namespace, name)
		}
		return nil, err
	}
	replicaSets, err := d.listDeploymentReplicaSets(ctx, deployment)
	if err != nil {
		return nil, err
	}
	records := make([]RevisionRecord, 0, len(replicaSets))
	for _, rs := range replicaSets {
		revision := rs.Annotations[deploymentRevisionAnnotation]
		if revision == "" {
			continue
		}
		expectedReplicas := int32(1)
		if rs.Spec.Replicas != nil {
			expectedReplicas = *rs.Spec.Replicas
		}
		healthy := rs.Status.ReadyReplicas >= expectedReplicas && rs.Status.AvailableReplicas >= 1
		records = append(records, RevisionRecord{
			Revision: revision,
			UnixTime: rs.CreationTimestamp.Unix(),
			Healthy:  healthy,
		})
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("no revisions found for deployment %s/%s", namespace, name)
	}
	return records, nil
}

func (d DeploymentAdapter) RollbackToRevision(ctx context.Context, namespace, name, revision string) error {
	if revision == "" {
		return fmt.Errorf("revision is required")
	}
	if d.Client == nil {
		return fmt.Errorf("kubernetes client is required")
	}
	deployment := appsv1.Deployment{}
	if err := d.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &deployment); err != nil {
		return err
	}
	replicaSets, err := d.listDeploymentReplicaSets(ctx, deployment)
	if err != nil {
		return err
	}
	for _, rs := range replicaSets {
		if rs.Annotations[deploymentRevisionAnnotation] != revision {
			continue
		}
		deployment.Spec.Template = rs.Spec.Template
		return d.Client.Update(ctx, &deployment)
	}
	return fmt.Errorf("revision %s not found for deployment %s/%s", revision, namespace, name)
}

func (d DeploymentAdapter) CountAffectedPods(ctx context.Context, namespace, name string) (int, error) {
	if d.Client == nil {
		return 0, fmt.Errorf("kubernetes client is required")
	}
	deployment := appsv1.Deployment{}
	if err := d.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &deployment); err != nil {
		return 0, err
	}
	if deployment.Status.Replicas > 0 {
		return int(deployment.Status.Replicas), nil
	}
	if deployment.Spec.Replicas != nil && *deployment.Spec.Replicas > 0 {
		return int(*deployment.Spec.Replicas), nil
	}
	return 1, nil
}

func (d DeploymentAdapter) CountClusterPods(ctx context.Context, namespace string) (int, error) {
	if d.Client == nil {
		return 0, fmt.Errorf("kubernetes client is required")
	}
	pods := corev1.PodList{}
	if err := d.Client.List(ctx, &pods, client.InNamespace(namespace)); err != nil {
		return 0, err
	}
	if len(pods.Items) == 0 {
		return 1, nil
	}
	return len(pods.Items), nil
}

func (d DeploymentAdapter) listDeploymentReplicaSets(ctx context.Context, deployment appsv1.Deployment) ([]appsv1.ReplicaSet, error) {
	replicaSetList := appsv1.ReplicaSetList{}
	if err := d.Client.List(ctx, &replicaSetList, client.InNamespace(deployment.Namespace)); err != nil {
		return nil, err
	}
	items := make([]appsv1.ReplicaSet, 0)
	for _, rs := range replicaSetList.Items {
		if metav1.IsControlledBy(&rs, &deployment) {
			items = append(items, rs)
		}
	}
	return items, nil
}
