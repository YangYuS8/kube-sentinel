package healing

import (
	"context"
	"fmt"
	"time"

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
	return kind == "Deployment" || kind == "StatefulSet"
}

func (d DeploymentAdapter) ListRevisions(ctx context.Context, namespace, name string) ([]RevisionRecord, error) {
	if d.Client == nil {
		return nil, fmt.Errorf("kubernetes client is required")
	}
	deployment := appsv1.Deployment{}
	if err := d.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &deployment); err == nil {
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
	} else if !apierrors.IsNotFound(err) {
		return nil, err
	}

	statefulSet := appsv1.StatefulSet{}
	if err := d.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &statefulSet); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("workload %s/%s not found as Deployment or StatefulSet", namespace, name)
		}
		return nil, err
	}
	revisions, err := d.listStatefulSetControllerRevisions(ctx, statefulSet)
	if err != nil {
		return nil, err
	}
	records := make([]RevisionRecord, 0, len(revisions))
	expectedReplicas := int32(1)
	if statefulSet.Spec.Replicas != nil {
		expectedReplicas = *statefulSet.Spec.Replicas
	}
	for _, revision := range revisions {
		revisionName := revision.Name
		if revisionName == "" {
			revisionName = fmt.Sprintf("rev-%d", revision.Revision)
		}
		healthy := statefulSet.Status.ReadyReplicas >= expectedReplicas
		records = append(records, RevisionRecord{
			Revision: revisionName,
			UnixTime: revision.CreationTimestamp.Unix(),
			Healthy:  healthy,
		})
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("no revisions found for statefulset %s/%s", namespace, name)
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
	if err := d.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &deployment); err == nil {
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
	} else if !apierrors.IsNotFound(err) {
		return err
	}
	statefulSet := appsv1.StatefulSet{}
	if err := d.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &statefulSet); err != nil {
		return err
	}
	revisions, err := d.listStatefulSetControllerRevisions(ctx, statefulSet)
	if err != nil {
		return err
	}
	found := false
	for _, controllerRevision := range revisions {
		if controllerRevision.Name == revision {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("revision %s not found for statefulset %s/%s", revision, namespace, name)
	}
	statefulSet.Annotations = ensureStringMap(statefulSet.Annotations)
	statefulSet.Annotations["kube-sentinel.io/rollback-target-revision"] = revision
	statefulSet.Spec.UpdateStrategy = appsv1.StatefulSetUpdateStrategy{Type: appsv1.RollingUpdateStatefulSetStrategyType}
	if statefulSet.Spec.UpdateStrategy.RollingUpdate == nil {
		statefulSet.Spec.UpdateStrategy.RollingUpdate = &appsv1.RollingUpdateStatefulSetStrategy{}
	}
	partition := int32(0)
	statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition = &partition
	return d.Client.Update(ctx, &statefulSet)
}

func (d DeploymentAdapter) ExecuteDeploymentControlledAction(ctx context.Context, namespace, name, actionType string) error {
	if d.Client == nil {
		return fmt.Errorf("kubernetes client is required")
	}
	if actionType != "rollout-restart" {
		return fmt.Errorf("unsupported deployment action type %q", actionType)
	}
	deployment := appsv1.Deployment{}
	if err := d.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &deployment); err != nil {
		return err
	}
	deployment.Spec.Template.Annotations = ensureStringMap(deployment.Spec.Template.Annotations)
	deployment.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().UTC().Format(time.RFC3339)
	return d.Client.Update(ctx, &deployment)
}

func (d DeploymentAdapter) ExecuteStatefulSetControlledAction(ctx context.Context, namespace, name, actionType string) error {
	if d.Client == nil {
		return fmt.Errorf("kubernetes client is required")
	}
	if actionType != "restart" {
		return fmt.Errorf("unsupported statefulset action type %q", actionType)
	}
	statefulSet := appsv1.StatefulSet{}
	if err := d.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &statefulSet); err != nil {
		return err
	}
	statefulSet.Spec.Template.Annotations = ensureStringMap(statefulSet.Spec.Template.Annotations)
	statefulSet.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().UTC().Format(time.RFC3339)
	return d.Client.Update(ctx, &statefulSet)
}

func (d DeploymentAdapter) ValidateStatefulSetEvidence(ctx context.Context, namespace, name string) error {
	if d.Client == nil {
		return fmt.Errorf("kubernetes client is required")
	}
	statefulSet := appsv1.StatefulSet{}
	if err := d.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &statefulSet); err != nil {
		return err
	}
	replicas := int32(1)
	if statefulSet.Spec.Replicas != nil {
		replicas = *statefulSet.Spec.Replicas
	}
	if statefulSet.Status.ReadyReplicas < 1 || statefulSet.Status.ReadyReplicas > replicas {
		return fmt.Errorf("statefulset readiness evidence is invalid")
	}
	return nil
}

func (d DeploymentAdapter) ValidateRevisionDependencies(ctx context.Context, namespace, name, revision string) error {
	if d.Client == nil {
		return fmt.Errorf("kubernetes client is required")
	}
	if revision == "" {
		return fmt.Errorf("revision is required")
	}
	deployment := appsv1.Deployment{}
	if err := d.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &deployment); err == nil {
		replicaSets, err := d.listDeploymentReplicaSets(ctx, deployment)
		if err != nil {
			return err
		}
		for _, rs := range replicaSets {
			if rs.Annotations[deploymentRevisionAnnotation] != revision {
				continue
			}
			for _, volume := range rs.Spec.Template.Spec.Volumes {
				if volume.ConfigMap != nil && volume.ConfigMap.Name != "" {
					obj := corev1.ConfigMap{}
					if err := d.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: volume.ConfigMap.Name}, &obj); err != nil {
						return fmt.Errorf("configmap dependency missing: %s", volume.ConfigMap.Name)
					}
				}
				if volume.Secret != nil && volume.Secret.SecretName != "" {
					obj := corev1.Secret{}
					if err := d.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: volume.Secret.SecretName}, &obj); err != nil {
						return fmt.Errorf("secret dependency missing: %s", volume.Secret.SecretName)
					}
				}
			}
			for _, container := range rs.Spec.Template.Spec.Containers {
				for _, envFrom := range container.EnvFrom {
					if envFrom.ConfigMapRef != nil && envFrom.ConfigMapRef.Name != "" {
						obj := corev1.ConfigMap{}
						if err := d.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: envFrom.ConfigMapRef.Name}, &obj); err != nil {
							return fmt.Errorf("configmap dependency missing: %s", envFrom.ConfigMapRef.Name)
						}
					}
					if envFrom.SecretRef != nil && envFrom.SecretRef.Name != "" {
						obj := corev1.Secret{}
						if err := d.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: envFrom.SecretRef.Name}, &obj); err != nil {
							return fmt.Errorf("secret dependency missing: %s", envFrom.SecretRef.Name)
						}
					}
				}
			}
			return nil
		}
		return fmt.Errorf("revision %s not found for deployment %s/%s", revision, namespace, name)
	} else if !apierrors.IsNotFound(err) {
		return err
	}
	statefulSet := appsv1.StatefulSet{}
	if err := d.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &statefulSet); err != nil {
		return err
	}
	revisions, err := d.listStatefulSetControllerRevisions(ctx, statefulSet)
	if err != nil {
		return err
	}
	revisionFound := false
	for _, controllerRevision := range revisions {
		if controllerRevision.Name != revision {
			continue
		}
		revisionFound = true
		break
	}
	if !revisionFound {
		return fmt.Errorf("revision %s not found for statefulset %s/%s", revision, namespace, name)
	}
	for _, volume := range statefulSet.Spec.Template.Spec.Volumes {
		if volume.ConfigMap != nil && volume.ConfigMap.Name != "" {
			obj := corev1.ConfigMap{}
			if err := d.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: volume.ConfigMap.Name}, &obj); err != nil {
				return fmt.Errorf("configmap dependency missing: %s", volume.ConfigMap.Name)
			}
		}
		if volume.Secret != nil && volume.Secret.SecretName != "" {
			obj := corev1.Secret{}
			if err := d.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: volume.Secret.SecretName}, &obj); err != nil {
				return fmt.Errorf("secret dependency missing: %s", volume.Secret.SecretName)
			}
		}
	}
	for _, container := range statefulSet.Spec.Template.Spec.Containers {
		for _, envFrom := range container.EnvFrom {
			if envFrom.ConfigMapRef != nil && envFrom.ConfigMapRef.Name != "" {
				obj := corev1.ConfigMap{}
				if err := d.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: envFrom.ConfigMapRef.Name}, &obj); err != nil {
					return fmt.Errorf("configmap dependency missing: %s", envFrom.ConfigMapRef.Name)
				}
			}
			if envFrom.SecretRef != nil && envFrom.SecretRef.Name != "" {
				obj := corev1.Secret{}
				if err := d.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: envFrom.SecretRef.Name}, &obj); err != nil {
					return fmt.Errorf("secret dependency missing: %s", envFrom.SecretRef.Name)
				}
			}
		}
	}
	return nil
}

func (d DeploymentAdapter) CountAffectedPods(ctx context.Context, namespace, name string) (int, error) {
	if d.Client == nil {
		return 0, fmt.Errorf("kubernetes client is required")
	}
	deployment := appsv1.Deployment{}
	if err := d.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &deployment); err == nil {
		if deployment.Status.Replicas > 0 {
			return int(deployment.Status.Replicas), nil
		}
		if deployment.Spec.Replicas != nil && *deployment.Spec.Replicas > 0 {
			return int(*deployment.Spec.Replicas), nil
		}
		return 1, nil
	}
	statefulSet := appsv1.StatefulSet{}
	if err := d.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &statefulSet); err == nil {
		if statefulSet.Status.Replicas > 0 {
			return int(statefulSet.Status.Replicas), nil
		}
		if statefulSet.Spec.Replicas != nil && *statefulSet.Spec.Replicas > 0 {
			return int(*statefulSet.Spec.Replicas), nil
		}
		return 1, nil
	}
	return 0, fmt.Errorf("workload %s/%s not found as Deployment or StatefulSet", namespace, name)
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

func (d DeploymentAdapter) CountTotalWorkloads(ctx context.Context, namespace string) (int, error) {
	if d.Client == nil {
		return 0, fmt.Errorf("kubernetes client is required")
	}
	deployments := appsv1.DeploymentList{}
	if err := d.Client.List(ctx, &deployments, client.InNamespace(namespace)); err != nil {
		return 0, err
	}
	statefulSets := appsv1.StatefulSetList{}
	if err := d.Client.List(ctx, &statefulSets, client.InNamespace(namespace)); err != nil {
		return 0, err
	}
	total := len(deployments.Items) + len(statefulSets.Items)
	if total == 0 {
		return 1, nil
	}
	return total, nil
}

func (d DeploymentAdapter) CountUnhealthyWorkloads(ctx context.Context, namespace string) (int, error) {
	if d.Client == nil {
		return 0, fmt.Errorf("kubernetes client is required")
	}
	deployments := appsv1.DeploymentList{}
	if err := d.Client.List(ctx, &deployments, client.InNamespace(namespace)); err != nil {
		return 0, err
	}
	statefulSets := appsv1.StatefulSetList{}
	if err := d.Client.List(ctx, &statefulSets, client.InNamespace(namespace)); err != nil {
		return 0, err
	}
	unhealthy := 0
	for _, dep := range deployments.Items {
		specReplicas := int32(1)
		if dep.Spec.Replicas != nil {
			specReplicas = *dep.Spec.Replicas
		}
		if dep.Status.AvailableReplicas < specReplicas {
			unhealthy++
		}
	}
	for _, st := range statefulSets.Items {
		specReplicas := int32(1)
		if st.Spec.Replicas != nil {
			specReplicas = *st.Spec.Replicas
		}
		if st.Status.ReadyReplicas < specReplicas {
			unhealthy++
		}
	}
	return unhealthy, nil
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

func ensureStringMap(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

func (d DeploymentAdapter) listStatefulSetControllerRevisions(ctx context.Context, statefulSet appsv1.StatefulSet) ([]appsv1.ControllerRevision, error) {
	revisionList := appsv1.ControllerRevisionList{}
	if err := d.Client.List(ctx, &revisionList, client.InNamespace(statefulSet.Namespace)); err != nil {
		return nil, err
	}
	items := make([]appsv1.ControllerRevision, 0)
	for _, revision := range revisionList.Items {
		if metav1.IsControlledBy(&revision, &statefulSet) {
			items = append(items, revision)
		}
	}
	return items, nil
}
