package healing

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDeploymentAdapterListRevisions(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default", UID: types.UID("dep-uid")},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "app"}},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "app"}}},
		},
	}
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-rs-1",
			Namespace: "default",
			Annotations: map[string]string{
				deploymentRevisionAnnotation: "3",
			},
			OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: "app", UID: dep.UID, Controller: ptrBool(true)}},
		},
		Spec:   appsv1.ReplicaSetSpec{Replicas: ptrInt32(2)},
		Status: appsv1.ReplicaSetStatus{ReadyReplicas: 2, AvailableReplicas: 2},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dep, rs).Build()
	adapter := NewDeploymentAdapter(cl)
	revs, err := adapter.ListRevisions(context.Background(), "default", "app")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(revs) != 1 || revs[0].Revision != "3" || !revs[0].Healthy {
		t.Fatalf("unexpected revisions result: %+v", revs)
	}
}

func TestDeploymentAdapterListRevisionsMarksHistoricalScaledDownReplicaSetHealthy(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app",
			Namespace: "default",
			UID:       types.UID("dep-uid"),
			Annotations: map[string]string{
				deploymentRevisionAnnotation: "2",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "app"}},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "app"}}},
		},
	}
	oldRS := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-rs-1",
			Namespace: "default",
			Annotations: map[string]string{
				deploymentRevisionAnnotation:                "1",
				"deployment.kubernetes.io/desired-replicas": "1",
			},
			OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: "app", UID: dep.UID, Controller: ptrBool(true)}},
		},
		Spec:   appsv1.ReplicaSetSpec{Replicas: ptrInt32(0)},
		Status: appsv1.ReplicaSetStatus{ReadyReplicas: 0, AvailableReplicas: 0},
	}
	currentRS := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-rs-2",
			Namespace: "default",
			Annotations: map[string]string{
				deploymentRevisionAnnotation:                "2",
				"deployment.kubernetes.io/desired-replicas": "1",
			},
			OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: "app", UID: dep.UID, Controller: ptrBool(true)}},
		},
		Spec:   appsv1.ReplicaSetSpec{Replicas: ptrInt32(1)},
		Status: appsv1.ReplicaSetStatus{ReadyReplicas: 1, AvailableReplicas: 1},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dep, oldRS, currentRS).Build()
	adapter := NewDeploymentAdapter(cl)
	revs, err := adapter.ListRevisions(context.Background(), "default", "app")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(revs) != 2 {
		t.Fatalf("expected two revisions, got %+v", revs)
	}
	if !revs[0].Healthy && !revs[1].Healthy {
		t.Fatalf("expected at least one healthy revision, got %+v", revs)
	}
	foundHistoricalHealthy := false
	for _, revision := range revs {
		if revision.Revision == "1" && revision.Healthy {
			foundHistoricalHealthy = true
		}
	}
	if !foundHistoricalHealthy {
		t.Fatalf("expected scaled-down historical revision to remain rollback-eligible, got %+v", revs)
	}
}

func TestDeploymentAdapterRollbackToRevision(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default", UID: types.UID("dep-uid")},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "app"}},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"version": "new"}}},
		},
	}
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-rs-1",
			Namespace: "default",
			Annotations: map[string]string{
				deploymentRevisionAnnotation: "2",
			},
			OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: "app", UID: dep.UID, Controller: ptrBool(true)}},
		},
		Spec: appsv1.ReplicaSetSpec{Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"version": "stable"}}}},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dep, rs).Build()
	adapter := NewDeploymentAdapter(cl)
	if err := adapter.RollbackToRevision(context.Background(), "default", "app", "2"); err != nil {
		t.Fatalf("unexpected rollback err: %v", err)
	}
	updated := &appsv1.Deployment{}
	if err := cl.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "app"}, updated); err != nil {
		t.Fatalf("get updated deployment failed: %v", err)
	}
	if updated.Spec.Template.Labels["version"] != "stable" {
		t.Fatalf("expected deployment template rollback to stable, got %s", updated.Spec.Template.Labels["version"])
	}
}

func TestDeploymentAdapterCountPods(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	replicas := int32(3)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default", UID: types.UID("dep-uid")},
		Spec:       appsv1.DeploymentSpec{Replicas: &replicas, Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "app"}}},
		Status:     appsv1.DeploymentStatus{Replicas: 3},
	}
	pod1 := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p1", Namespace: "default"}}
	pod2 := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p2", Namespace: "default"}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dep, pod1, pod2).Build()
	adapter := NewDeploymentAdapter(cl)
	affected, err := adapter.CountAffectedPods(context.Background(), "default", "app")
	if err != nil {
		t.Fatalf("count affected failed: %v", err)
	}
	if affected != 3 {
		t.Fatalf("expected affected pods 3, got %d", affected)
	}
	cluster, err := adapter.CountClusterPods(context.Background(), "default")
	if err != nil {
		t.Fatalf("count cluster pods failed: %v", err)
	}
	if cluster != 2 {
		t.Fatalf("expected cluster pod count 2, got %d", cluster)
	}
}

func TestDeploymentAdapterValidateRevisionDependencies(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default", UID: types.UID("dep-uid")},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "app"}},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"version": "new"}}},
		},
	}
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-rs-1",
			Namespace: "default",
			Annotations: map[string]string{
				deploymentRevisionAnnotation: "2",
			},
			OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: "app", UID: dep.UID, Controller: ptrBool(true)}},
		},
		Spec: appsv1.ReplicaSetSpec{Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Volumes:    []corev1.Volume{{Name: "cfg", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cfg"}}}}},
				Containers: []corev1.Container{{Name: "app", EnvFrom: []corev1.EnvFromSource{{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "sec"}}}}}},
			},
		}},
	}
	cfg := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cfg", Namespace: "default"}}
	sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "sec", Namespace: "default"}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dep, rs, cfg, sec).Build()
	adapter := NewDeploymentAdapter(cl)
	if err := adapter.ValidateRevisionDependencies(context.Background(), "default", "app", "2"); err != nil {
		t.Fatalf("expected dependencies valid, got %v", err)
	}
}

func TestDeploymentAdapterListRevisionsExcludesCurrentStatefulSetRevisionFromHealthyCandidates(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default", UID: types.UID("sts-uid")},
		Spec:       appsv1.StatefulSetSpec{Replicas: ptrInt32(1)},
		Status: appsv1.StatefulSetStatus{
			ReadyReplicas:   1,
			CurrentRevision: "app-rev-2",
			UpdateRevision:  "app-rev-2",
		},
	}
	current := newStatefulSetControllerRevision(t, statefulSet, "app-rev-2", 2, corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"kube-sentinel.io/test-rev": "2"}},
	})
	historical := newStatefulSetControllerRevision(t, statefulSet, "app-rev-1", 1, corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"kube-sentinel.io/test-rev": "1"}},
	})
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(statefulSet, current, historical).Build()
	adapter := NewDeploymentAdapter(cl)
	revs, err := adapter.ListRevisions(context.Background(), "default", "app")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(revs) != 2 {
		t.Fatalf("expected two revisions, got %+v", revs)
	}
	for _, revision := range revs {
		switch revision.Revision {
		case "app-rev-2":
			if revision.Healthy {
				t.Fatalf("expected current statefulset revision to be excluded from healthy rollback candidates, got %+v", revs)
			}
		case "app-rev-1":
			if !revision.Healthy {
				t.Fatalf("expected historical statefulset revision to remain rollback eligible, got %+v", revs)
			}
		}
	}
}

func TestDeploymentAdapterRollbackToRevisionRestoresStatefulSetTemplate(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default", UID: types.UID("sts-uid")},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    ptrInt32(1),
			ServiceName: "app",
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"kube-sentinel.io/test-rev": "2"}},
				Spec: corev1.PodSpec{Containers: []corev1.Container{{
					Name:  "app",
					Image: "nginx:1.27-alpine",
					EnvFrom: []corev1.EnvFromSource{{
						ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cfg-v2"}},
					}},
				}}},
			},
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{Type: appsv1.RollingUpdateStatefulSetStrategyType},
		},
	}
	revision := newStatefulSetControllerRevision(t, statefulSet, "app-rev-1", 1, corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"kube-sentinel.io/test-rev": "1"}},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{
			Name:  "app",
			Image: "nginx:1.27-alpine",
			EnvFrom: []corev1.EnvFromSource{{
				ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cfg-v1"}},
			}},
		}}},
	})
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(statefulSet, revision).Build()
	adapter := NewDeploymentAdapter(cl)
	if err := adapter.RollbackToRevision(context.Background(), "default", "app", "app-rev-1"); err != nil {
		t.Fatalf("unexpected rollback err: %v", err)
	}
	updated := &appsv1.StatefulSet{}
	if err := cl.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "app"}, updated); err != nil {
		t.Fatalf("get updated statefulset failed: %v", err)
	}
	if got := updated.Spec.Template.Annotations["kube-sentinel.io/test-rev"]; got != "1" {
		t.Fatalf("expected statefulset template rollback to rev 1, got %s", got)
	}
	if len(updated.Spec.Template.Spec.Containers) == 0 || len(updated.Spec.Template.Spec.Containers[0].EnvFrom) == 0 || updated.Spec.Template.Spec.Containers[0].EnvFrom[0].ConfigMapRef == nil {
		t.Fatalf("expected statefulset envFrom to be restored from controller revision")
	}
	if got := updated.Spec.Template.Spec.Containers[0].EnvFrom[0].ConfigMapRef.Name; got != "cfg-v1" {
		t.Fatalf("expected rollback to cfg-v1, got %s", got)
	}
	if got := updated.Annotations["kube-sentinel.io/rollback-target-revision"]; got != "app-rev-1" {
		t.Fatalf("expected rollback target annotation to be recorded, got %s", got)
	}
	if updated.Spec.UpdateStrategy.Type != appsv1.RollingUpdateStatefulSetStrategyType || updated.Spec.UpdateStrategy.RollingUpdate == nil || updated.Spec.UpdateStrategy.RollingUpdate.Partition == nil || *updated.Spec.UpdateStrategy.RollingUpdate.Partition != 0 {
		t.Fatalf("expected rollback to retain a full rolling update strategy, got %+v", updated.Spec.UpdateStrategy)
	}
}

func TestDeploymentAdapterValidateRevisionDependenciesUsesTargetStatefulSetRevision(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default", UID: types.UID("sts-uid")},
		Spec: appsv1.StatefulSetSpec{
			Replicas: ptrInt32(1),
			Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{
				Name:  "app",
				Image: "nginx:1.27-alpine",
				EnvFrom: []corev1.EnvFromSource{{
					ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cfg-v2"}},
				}},
			}}}},
		},
	}
	revision := newStatefulSetControllerRevision(t, statefulSet, "app-rev-1", 1, corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{
		Name:  "app",
		Image: "nginx:1.27-alpine",
		EnvFrom: []corev1.EnvFromSource{{
			ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cfg-v1"}},
		}},
	}}}})
	cfgV2 := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cfg-v2", Namespace: "default"}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(statefulSet, revision, cfgV2).Build()
	adapter := NewDeploymentAdapter(cl)
	err := adapter.ValidateRevisionDependencies(context.Background(), "default", "app", "app-rev-1")
	if err == nil {
		t.Fatalf("expected missing target revision dependency to fail")
	}
	if !strings.Contains(err.Error(), "cfg-v1") {
		t.Fatalf("expected missing dependency to reference cfg-v1, got %v", err)
	}
}

func TestDeploymentAdapterCountWorkloads(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	dep1Replicas := int32(3)
	dep2Replicas := int32(2)
	dep1 := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "app1", Namespace: "default"}, Spec: appsv1.DeploymentSpec{Replicas: &dep1Replicas}, Status: appsv1.DeploymentStatus{AvailableReplicas: 1}}
	dep2 := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "app2", Namespace: "default"}, Spec: appsv1.DeploymentSpec{Replicas: &dep2Replicas}, Status: appsv1.DeploymentStatus{AvailableReplicas: 2}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dep1, dep2).Build()
	adapter := NewDeploymentAdapter(cl)
	total, err := adapter.CountTotalWorkloads(context.Background(), "default")
	if err != nil || total != 2 {
		t.Fatalf("expected total workloads 2, got %d err=%v", total, err)
	}
	unhealthy, err := adapter.CountUnhealthyWorkloads(context.Background(), "default")
	if err != nil || unhealthy != 1 {
		t.Fatalf("expected unhealthy workloads 1, got %d err=%v", unhealthy, err)
	}
}

func newStatefulSetControllerRevision(t *testing.T, statefulSet *appsv1.StatefulSet, name string, revision int64, template corev1.PodTemplateSpec) *appsv1.ControllerRevision {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"spec": map[string]any{
			"template":       template,
			"updateStrategy": appsv1.StatefulSetUpdateStrategy{Type: appsv1.RollingUpdateStatefulSetStrategyType},
		},
	})
	if err != nil {
		t.Fatalf("marshal controller revision payload: %v", err)
	}
	return &appsv1.ControllerRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: statefulSet.Namespace,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "apps/v1",
				Kind:       "StatefulSet",
				Name:       statefulSet.Name,
				UID:        statefulSet.UID,
				Controller: ptrBool(true),
			}},
		},
		Revision: revision,
		Data:     runtime.RawExtension{Raw: raw},
	}
}

func ptrBool(v bool) *bool    { return &v }
func ptrInt32(v int32) *int32 { return &v }
